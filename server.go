package server

import (
	"errors"
	"fmt"
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

type Opt func(*Server)

// WithUnsafeClient allows the HTTP client to be set. No client other than the
// default is safe to use.
func WithUnsafeClient(c *http.Client) Opt {
	return func(s *Server) {
		s.client = c
	}
}

// MaxConns sets the total max upstream connections the server can make. The
// default is 8.
func MaxConns(c int) Opt {
	return func(s *Server) {
		s.semaphore = make(chan struct{}, c)
	}
}

func WithClock(f func() time.Time) Opt {
	return func(s *Server) {
		s.now = f
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
			Transport: &http.Transport{
				DialContext: publicUnicastOnlyDialContext,
			},
			CheckRedirect: noRedirect,
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
	if upstream.Header.Get("Content-Type") != "text/calendar" {
		return nil, errors.New("not a calendar")
	}

	return ics.ParseCalendar(upstream.Body)
}

func (s *Server) HandleWebcal(c *gin.Context) {
	if isBrowser(c) {
		s.HandleIndex(c)
		return
	}

	upstreamURLString := c.Query("cal")
	upstreamURL, err := s.parseCalendarURL(upstreamURLString)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error validating calendar URL: %s", err))
		return
	}

	includes, err := parseMatchers(c.QueryArray("inc"))
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error parsing inc argument %q: %s", c.QueryArray("inc"), err))
		return
	}
	if len(includes) == 0 {
		includes = append(includes, matcher{
			property:   ics.ComponentPropertySummary,
			expression: regexp.MustCompile(".*"),
		})
	}
	excludes, err := parseMatchers(c.QueryArray("exc"))
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error parsing exc argument %q: %s", c.QueryArray("exc"), err))
		return
	}

	var merge bool
	if mergeArg := c.Query("mrg"); mergeArg != "" {
		merge, err = strconv.ParseBool(mergeArg)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("error parsing mrg argument: %s", err))
			return
		}
	}

	upstream, err := s.fetch(upstreamURL)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadGateway,
			fmt.Errorf("error fetching calendar %q: %s", upstreamURL, err))
		return
	}

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
		if includes.matches(event) && !excludes.matches(event) {
			events = append(events, event)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		startI, sErr := events[i].GetStartAt()
		if sErr != nil {
			err = fmt.Errorf("event %q has no start date: %w", events[i].Serialize(), sErr)
			return false
		}
		startJ, sErr := events[j].GetStartAt()
		if sErr != nil {
			err = fmt.Errorf("event %q has no start date: %w", events[i].Serialize(), sErr)
			return false
		}
		return startI.Before(startJ)
	})
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error sorting events: %s", err))
		return
	}

	if merge {
		events, err = mergeEvents(events)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("error merging events: %s", err))
			return
		}
	}

	for _, event := range events {
		downstream.AddVEvent(event)
	}

	c.Header("Content-Type", "text/calendar")
	_ = downstream.SerializeTo(c.Writer)
}

func (s *Server) HandleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index", nil)
}

func isBrowser(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}

	for _, mediaType := range strings.Split(c.GetHeader("Accept"), ",") {
		if mediaType == "text/html" {
			return true
		}
	}
	return false
}

func (s *Server) HandleCalendar(c *gin.Context) {
	userTime := s.now().UTC()

	tz, err := time.LoadLocation(c.PostForm("user-tz"))
	if nil == err {
		log(c).Debugf("Using user time zone %q", tz)
		userTime = userTime.In(tz)
	} else {
		log(c).Warnf("Failed to parse user time zone: %s, using UTC.", err)
	}

	c.HTML(http.StatusOK, "calendar", newCalendar(ViewMonth, userTime))
}
