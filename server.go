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
			Transport: &http.Transport{
				DialContext: publicUnicastOnlyDialContext,
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

func (s *Server) HandleWebcal(c *gin.Context) {
	if isBrowser(c) {
		s.HandleIndex(c)
		return
	}

	downstream, err := s.getProcessedCal(c,
		c.Query("cal"),
		c.Query("mrg"),
		c.QueryArray("inc"),
		c.QueryArray("exc"),
	)
	if err != nil {
		// getProcessedCal aborts all errors
		return
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
	var (
		downstream *ics.Calendar
		err        error
	)
	if cal := c.PostForm("cal"); cal != "" {
		downstream, err = s.getProcessedCal(c,
			cal,
			c.PostForm("mrg"),
			c.PostFormArray("inc"),
			c.PostFormArray("exc"),
		)
		if err != nil {
			// getProcessedCal aborts all errors
			return
		}
	}

	userTime := s.now().UTC()
	tz, err := time.LoadLocation(c.PostForm("user-tz"))
	if nil == err {
		log(c).Debugf("Using user time zone %q", tz)
		userTime = userTime.In(tz)
	} else {
		log(c).Warnf("Failed to parse user time zone: %s, using UTC.", err)
	}

	c.HTML(http.StatusOK, "calendar", newCalendar(c, ViewMonth, userTime, downstream))
}

// TODO move some funcs around
func (s *Server) getProcessedCal(c *gin.Context, url, mrg string, inc, exc []string) (*ics.Calendar, error) {
	upstreamURL, err := s.parseCalendarURL(url)
	if err != nil {
		err = fmt.Errorf("error validating calendar URL: %s", err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return nil, err
	}

	includes, err := parseMatchers(inc)
	if err != nil {
		err = fmt.Errorf("error parsing inc argument %q: %s", c.QueryArray("inc"), err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return nil, err
	}
	if len(includes) == 0 {
		includes = append(includes, matcher{
			property:   ics.ComponentPropertySummary,
			expression: regexp.MustCompile(".*"),
		})
	}
	excludes, err := parseMatchers(exc)
	if err != nil {
		err = fmt.Errorf("error parsing exc argument %q: %s", c.QueryArray("exc"), err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return nil, err
	}

	var merge bool
	if mrg != "" {
		merge, err = strconv.ParseBool(mrg)
		if err != nil {
			err = fmt.Errorf("error parsing mrg argument: %s", err)
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return nil, err
		}
	}

	upstream, err := s.fetch(upstreamURL)
	if err != nil {
		err = fmt.Errorf("error fetching calendar %q: %s", upstreamURL, err)
		_ = c.AbortWithError(http.StatusBadGateway, err)
		return nil, err
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
		err = fmt.Errorf("error sorting events: %s", err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return nil, err
	}

	if merge {
		events, err = mergeEvents(events)
		if err != nil {
			err = fmt.Errorf("error merging events: %s", err)
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return nil, err
		}
	}

	for _, event := range events {
		downstream.AddVEvent(event)
	}

	return downstream, nil
}
