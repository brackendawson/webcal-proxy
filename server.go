package server

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/brackendawson/webcal-proxy/assets"
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

	r.Use(logging)
	r.SetHTMLTemplate(assets.Templates())
	r.GET("/", s.HandleWebcal)
	r.POST("/", s.HandleCalendar)
	r.StaticFS("/assets", http.FS(assets.Assets))

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// TODO move things to the right file
type calenderOptions struct {
	url                string
	includes, excludes matchGroup
	merge              bool
}

func (s *Server) HandleWebcal(c *gin.Context) {
	if isBrowser(c) {
		s.HandleIndex(c)
		return
	}

	opts, err := getCalendarOptions(c, c.QueryArray)
	if err != nil {
		handleWebcalErr(c, err)
		return
	}

	upstream, err := s.getUpstreamCalendar(c, opts.url)
	if err != nil {
		handleWebcalErr(c, err)
		return
	}

	downstream := s.getDownstreamCalendar(c, upstream, opts)

	c.Header("Content-Type", "text/calendar")
	_ = downstream.SerializeTo(c.Writer)
}

// TODO move
func handleWebcalErr(c *gin.Context, err error) {
	var msgErr errorWithMessage
	if errors.As(err, &msgErr) {
		c.String(msgErr.code, msgErr.message)
		return
	}

	log(c).Error(err)

	handleWebcalErr(c, newErrorWithMessage(
		http.StatusInternalServerError,
		"%s", http.StatusText(http.StatusInternalServerError),
	))
}

func isBrowser(c *gin.Context) bool {
	for _, mediaType := range strings.Split(c.GetHeader("Accept"), ",") {
		if mediaType == "text/html" {
			return true
		}
	}
	return false
}

func (s *Server) HandleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index", nil)
}

func (s *Server) HandleCalendar(c *gin.Context) {
	now := s.now().UTC()
	tz, err := time.LoadLocation(c.PostForm("user-tz"))
	if nil == err {
		log(c).Debugf("Using user time zone %q", tz)
		now = now.In(tz)
	} else {
		log(c).Warnf("Failed to parse user time zone: %s, using UTC.", err)
	}

	opts, err := getCalendarOptions(c, c.PostFormArray)
	if err != nil {
		handleCalendarError(c, newCalendar(c, ViewMonth, now, nil), err)
		return
	}

	if opts.url == "" {
		c.HTML(http.StatusOK, "calendar", newCalendar(c, ViewMonth, now, nil))
		return
	}

	upstream, upstreamFromCache, err := s.getUpstreamWithCache(c, opts.url)
	if err != nil {
		handleCalendarError(c, newCalendar(c, ViewMonth, now, nil), err)
		return
	}

	downstream := s.getDownstreamCalendar(c, upstream, opts)

	calendar := newCalendar(c, ViewMonth, now, downstream)

	if upstream != nil && !upstreamFromCache {
		calendar.Cache = &Cache{
			URL:      opts.url,
			Calendar: upstream,
		}
	}

	c.HTML(http.StatusOK, "calendar", calendar)
}

func (s *Server) getUpstreamCalendar(c *gin.Context, url string) (*ics.Calendar, error) {
	upstreamURL, err := parseURLScheme(c, url)
	if err != nil {
		return nil, err
	}

	upstream, err := s.fetch(upstreamURL)
	if err != nil {
		log(c).Warnf("Failed to fetch calendar %q: %s", upstreamURL, err)
		return nil, newErrorWithMessage(
			http.StatusBadGateway,
			"Failed to fetch calendar",
		)
	}

	return upstream, nil
}

func getCache(c *gin.Context, upstreamURL string) (*ics.Calendar, bool) {
	rawCache := c.PostForm("ical-cache")
	if rawCache == "" {
		return nil, false
	}
	cache, err := ParseCache(rawCache)
	if err != nil {
		log(c).Warnf("Failed to parse cache: %s. Continuing without.")
		return nil, false
	}

	if cache.URL != upstreamURL {
		log(c).Debugf("Cache was for old URL %q, ignoring.", cache.URL)
		return nil, false
	}

	log(c).Debug("Using cached calendar")
	return cache.Calendar, true
}

func (s *Server) getUpstreamWithCache(c *gin.Context, upstreamURL string) (_ *ics.Calendar, usedCache bool, _ error) {
	upstream, ok := getCache(c, upstreamURL)
	if !ok {
		upstream, err := s.getUpstreamCalendar(c, upstreamURL)
		if err != nil {
			return nil, false, err
		}

		return upstream, false, nil
	}

	return upstream, true, nil
}

func (s *Server) getDownstreamCalendar(c *gin.Context, upstream *ics.Calendar, opts calenderOptions) *ics.Calendar {
	downstream := ics.NewCalendar()

	for _, component := range upstream.Components {
		if _, ok := component.(*ics.VEvent); ok {
			continue
		}
		downstream.Components = append(downstream.Components, component)
	}
	downstream.CalendarProperties = upstream.CalendarProperties

	var events []*ics.VEvent
	for _, event := range upstream.Events() {
		if opts.includes.matches(event) && !opts.excludes.matches(event) {
			events = append(events, event)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		startI, _ := events[i].GetStartAt()
		startJ, _ := events[j].GetStartAt()
		return startI.Before(startJ)
	})

	if opts.merge {
		events = mergeEvents(events)
	}

	for _, event := range events {
		downstream.AddVEvent(event)
	}

	return downstream
}

// fetch fetches the given url from the given IP address
func (s *Server) fetch(url string) (*ics.Calendar, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	upstream, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer upstream.Body.Close()
	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		return nil, fmt.Errorf("bad status: %s", upstream.Status)
	}
	mediaType, _, err := mime.ParseMediaType(upstream.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("error parsing content type: %w", err)
	}
	if mediaType != "text/calendar" {
		return nil, errors.New("not a calendar")
	}

	return ics.ParseCalendar(upstream.Body)
}

// TODO move me
type errorWithMessage struct {
	code    int
	message string
}

func newErrorWithMessage(code int, format string, args ...any) errorWithMessage {
	return errorWithMessage{
		code:    code,
		message: fmt.Sprintf(format, args...),
	}
}

func (e errorWithMessage) Error() string {
	return e.message
}

// TODO move me
func getCalendarOptions(c *gin.Context, getArray func(string) []string) (calenderOptions, error) {
	var (
		opts calenderOptions
		err  error
	)

	opts.merge, err = getBool(c, getArray, "mrg")
	if err != nil {
		return calenderOptions{}, err
	}

	opts.includes, err = parseMatchers(getArray("inc"))
	if err != nil {
		return calenderOptions{}, newErrorWithMessage(
			http.StatusBadRequest,
			"Bad inc argument: %s", err.Error(),
		)
	}
	if len(opts.includes) == 0 {
		opts.includes = matchGroup{
			{
				property:   ics.ComponentPropertySummary,
				expression: regexp.MustCompile(".*"),
			},
		}
	}

	opts.excludes, err = parseMatchers(getArray("exc"))
	if err != nil {
		return calenderOptions{}, newErrorWithMessage(
			http.StatusBadRequest,
			"Bad exc argument: %s", err.Error(),
		)
	}

	opts.url = getString(getArray, "cal")

	return opts, nil
}

func getBool(c *gin.Context, getArray func(string) []string, key string) (bool, error) {
	bs := getArray(key)
	if len(bs) < 1 {
		return false, nil
	}

	b, err := strconv.ParseBool(bs[0])
	if err != nil {
		log(c).Warnf("error getting %q parameter: %w", key, err)
		return false, newErrorWithMessage(
			http.StatusBadRequest,
			"Bad argument %q for %q, should be boolean.", bs[0], key,
		)
	}

	return b, nil
}

func getString(getArray func(string) []string, key string) string {
	ss := getArray(key)
	if len(ss) < 1 {
		return ""
	}
	return ss[0]
}

// TODO move
// TODO rename all these, maybe HTMX?
func handleCalendarError(c *gin.Context, calendar Calendar, err error) {
	var msgErr errorWithMessage
	if errors.As(err, &msgErr) {
		calendar.Error = msgErr.message
		c.HTML(http.StatusOK, "calendar", calendar)
		return
	}

	log(c).Error(err)

	handleCalendarError(c, calendar, newErrorWithMessage(
		http.StatusInternalServerError,
		"%s", http.StatusText(http.StatusInternalServerError),
	))
}
