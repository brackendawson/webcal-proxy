package server

import (
	"fmt"
	"net/http"
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

	r.Use(logging)
	r.SetHTMLTemplate(assets.Templates())
	r.GET("/", s.HandleWebcal)
	r.POST("/", s.HandleHTMX)
	r.StaticFS("/assets", http.FS(assets.Assets))

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Server) HandleWebcal(c *gin.Context) {
	if isBrowser(c) {
		c.HTML(http.StatusOK, "index", struct {
			Host string
		}{
			Host: c.Request.Host,
		})
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

	downstream := getDownstreamCalendar(upstream, opts)

	c.Header("Content-Type", "text/calendar")
	_ = downstream.SerializeTo(c.Writer)
}

func (s *Server) HandleHTMX(c *gin.Context) {
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
		handleHTMXError(c, newCalendar(c, ViewMonth, now, nil), err)
		return
	}

	if opts.url == "" {
		c.HTML(http.StatusOK, "calendar", newCalendar(c, ViewMonth, now, nil))
		return
	}

	upstream, upstreamFromCache, err := s.getUpstreamWithCache(c, opts.url)
	if err != nil {
		handleHTMXError(c, newCalendar(c, ViewMonth, now, nil), err)
		return
	}

	downstream := getDownstreamCalendar(upstream, opts)

	calendar := newCalendar(c, ViewMonth, now, downstream)

	if upstream != nil && !upstreamFromCache {
		calendar.Cache = &cache.Webcal{
			URL:      opts.url,
			Calendar: upstream,
		}
	}

	calendar.URL = clientURL(c, opts).String()

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
