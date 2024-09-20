package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brackendawson/webcal-proxy/assets"
	"github.com/brackendawson/webcal-proxy/cache"
	"github.com/gin-gonic/gin"
)

const (
	defaultMaxConns = 8
)

var (
	serverName    = "webcal-proxy"
	serverVersion = "1.2.0+dev"
)

type Opt func(*Server)

// MaxConns sets the total max upstream connections the server can make. The
// default is 8.
func MaxConns(c int) Opt {
	return func(s *Server) {
		s.semaphore = make(chan struct{}, c)
	}
}

type Server struct {
	client    *http.Client
	semaphore chan struct{}

	now func() time.Time
}

func New(r *gin.Engine, opts ...Opt) *Server {
	s := &Server{
		client: &http.Client{
			Timeout: requestTimeoutSecs * time.Second,
			Transport: &WithUserAgent{
				RoundTripper: &http.Transport{
					DialContext: publicUnicastOnlyDialContext,
				},
				UserAgent: fmt.Sprintf("%s/%s", serverName, serverVersion),
			},
		},
		semaphore: make(chan struct{}, defaultMaxConns),
		now:       time.Now,
	}

	r.ContextWithFallback = true
	r.Use(logging)

	r.SetHTMLTemplate(assets.Templates())
	r.GET("/", s.HandleWebcal)
	r.POST("/", s.HandleHTMX)
	r.GET("/matcher", s.HandleMatcher)
	r.DELETE("/matcher", s.HandleMatcherDelete)
	r.GET("/date-picker-month", s.HandleDatePickerMonth)
	r.StaticFS("/assets", http.FS(assets.Assets))

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type View struct {
	ArgHost, ArgProxyPath string
}

func newView(c *gin.Context) View {
	host := c.Request.Host
	if host == "" {
		host = c.GetHeader("X-HX-Host")
	}

	return View{
		ArgHost:      host,
		ArgProxyPath: c.GetHeader("X-Forwarded-URI"),
	}
}

func (v View) Host() string {
	return v.ArgHost
}

func (v View) ProxyPath() string {
	return v.ArgProxyPath
}

type Index struct {
	View
	Options Options
	Error   string
}

func newIndex(c *gin.Context) Index {
	i := Index{
		View: newView(c),
	}
	opts, err := getCalendarOptions(c, c.QueryArray)
	if err != nil {
		i.Error = err.Error() + " Enter your webcal URL."
		return i
	}
	i.Options = opts.Options()
	return i
}

func (s *Server) HandleWebcal(c *gin.Context) {
	if isBrowser(c) {
		c.HTML(http.StatusOK, "index", newIndex(c))
		return
	}

	opts, err := getCalendarOptions(c, c.QueryArray)
	if err != nil {
		handleWebcalErr(c, err)
		return
	}
	if opts.url == "" {
		handleWebcalErr(c, newErrorWithMessage(
			http.StatusBadRequest,
			`Missing "cal" parameter, must be a webcal URL.`,
		))
		return
	}

	upstream, err := s.getUpstreamCalendar(c, opts.url)
	if err != nil {
		handleWebcalErr(c, err)
		return
	}

	downstream := getDownstreamCalendar(upstream, opts)

	c.Header("Content-Type", "text/calendar")
	_ = downstream.SerializeTo(c.Writer)
}

func (s *Server) HandleHTMX(c *gin.Context) {
	today := s.now().UTC()
	tz, err := time.LoadLocation(c.PostForm("user-tz"))
	if nil == err {
		log(c).Debugf("Using user time zone %q", tz)
		today = today.In(tz)
	} else {
		log(c).Warnf("Failed to parse user time zone: %s, using UTC.", err)
	}
	log(c).Debug("Using today: %s", today)

	target := today
	if t, ok := parseTarget(c, today); ok {
		target = t
	}
	log(c).Debug("Using target: %s", target)

	opts, err := getCalendarOptions(c, c.PostFormArray)
	if err != nil {
		handleHTMXError(c, newCalendar(c, newView(c), ViewMonth, target, today, nil), err)
		return
	}

	if opts.url == "" {
		c.HTML(http.StatusOK, "calendar", newCalendar(c, newView(c), ViewMonth, target, today, nil))
		return
	}

	upstream, upstreamFromCache, err := s.getUpstreamWithCache(c, opts.url, c.PostForm("ical-cache"))
	if err != nil {
		handleHTMXError(c, newCalendar(c, newView(c), ViewMonth, target, today, nil), err)
		return
	}

	downstream := getDownstreamCalendar(upstream, opts)

	calendar := newCalendar(c, newView(c), ViewMonth, target, today, downstream)

	if upstream != nil && !upstreamFromCache {
		calendar.Cache = &cache.Webcal{
			URL:      opts.url,
			Calendar: upstream,
		}
	}

	calendar.URL = clientURL(c).String()

	c.HTML(http.StatusOK, "calendar", calendar)
}

func isBrowser(c *gin.Context) bool {
	for _, mediaType := range strings.Split(c.GetHeader("Accept"), ",") {
		if mediaType == "text/html" {
			return true
		}
	}
	return false
}

func parseTarget(c *gin.Context, today time.Time) (time.Time, bool) {
	targetYear := c.PostForm("target-year")
	if targetYear == "" {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(targetYear)
	if err != nil {
		log(c).Error("Error parsing target-year: %s", err)
		return time.Time{}, false
	}

	targetMonth := c.PostForm("target-month")
	if targetMonth == "" {
		return time.Time{}, false
	}
	month, err := strconv.Atoi(targetMonth)
	if err != nil {
		log(c).Error("Error parsing target-month: %s", err)
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, today.Location()), true
}

func (s *Server) HandleMatcher(c *gin.Context) {
	c.HTML(http.StatusOK, "template-matcher-group", newView(c))
}

func (s *Server) HandleMatcherDelete(c *gin.Context) {
	// HTMX forms that handle click events from buttons swallow the events
	// before they can reach the browser. This means checkboxes don't work.
	// In order to submit the form after someone clicks the delete button next
	// to a matcher we trigger an input on the hidden #trigger-submit input.
	triggerFormSubmit(c)
}

func (s *Server) HandleDatePickerMonth(c *gin.Context) {
	t, err := time.Parse(time.RFC3339, c.Query("date"))
	if err != nil {
		handleWebcalErr(c, err)
		return
	}
	triggerFormSubmit(c)
	c.HTML(http.StatusOK, "date-picker-month", t)
}

func triggerFormSubmit(c *gin.Context) {
	c.Header("HX-Trigger-After-Settle", `{"input":{"target":"#trigger-submit"}}`)
}
