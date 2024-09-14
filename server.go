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

func (s *Server) HandleWebcal(c *gin.Context) {
	if isBrowser(c) {
		s.HandleIndex(c)
		return
	}

	merge, includes, excludes, ok := parseCalendarOptions(c)
	if !ok {
		return
	}

	upstream, ok := s.getUpstreamCalendar(c, c.Query("cal"))
	if !ok {
		return
	}

	downstream, ok := s.getDownstreamCalendar(c, upstream, merge, includes, excludes)
	if !ok {
		return
	}

	c.Header("Content-Type", "text/calendar")
	_ = downstream.SerializeTo(c.Writer)
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
	var (
		downstream *ics.Calendar
		err        error
	)
	upstreamURL := c.PostForm("cal")
	var (
		upstream  *ics.Calendar
		usedCache bool
	)
	if upstreamURL != "" {
		merge, includes, excludes, ok := parseCalendarOptions(c)
		if !ok {
			return
		}

		upstream, usedCache, ok = s.getUpstreamWithCache(c, upstreamURL)
		if !ok {
			return
		}

		if downstream, ok = s.getDownstreamCalendar(c, upstream, merge, includes, excludes); !ok {
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

	calendar := newCalendar(c, ViewMonth, userTime, downstream)

	if upstream != nil && !usedCache {
		calendar.Cache = &Cache{
			URL:      upstreamURL,
			Calendar: upstream,
		}
	}

	c.HTML(http.StatusOK, "calendar", calendar)
}

func (s *Server) getUpstreamCalendar(c *gin.Context, url string) (*ics.Calendar, bool) {
	upstreamURL, err := s.parseCalendarURL(url)
	if err != nil {
		err = fmt.Errorf("error validating calendar URL: %s", err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return nil, false
	}

	upstream, err := s.fetch(upstreamURL)
	if err != nil {
		err = fmt.Errorf("error fetching calendar %q: %s", upstreamURL, err)
		_ = c.AbortWithError(http.StatusBadGateway, err)
		return nil, false
	}

	return upstream, true
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

func (s *Server) getUpstreamWithCache(c *gin.Context, upstreamURL string) (_ *ics.Calendar, usedCache, ok bool) {
	upstream, ok := getCache(c, upstreamURL)
	if !ok {
		if upstream, ok = s.getUpstreamCalendar(c, upstreamURL); !ok {
			return nil, false, false
		}

		return upstream, false, true
	}

	return upstream, true, true
}

func (s *Server) getDownstreamCalendar(c *gin.Context, upstream *ics.Calendar, merge bool, includes, excludes matchGroup) (*ics.Calendar, bool) {
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
	var err error
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
		return nil, false
	}

	if merge {
		events, err = mergeEvents(events)
		if err != nil {
			err = fmt.Errorf("error merging events: %s", err)
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return nil, false
		}
	}

	for _, event := range events {
		downstream.AddVEvent(event)
	}

	return downstream, true
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

func parseMergeOption(c *gin.Context, mrg string) (_ bool, ok bool) {
	if mrg == "" {
		return false, true
	}

	merge, err := strconv.ParseBool(mrg)
	if err != nil {
		err = fmt.Errorf("error parsing mrg argument: %s", err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return false, false
	}

	return merge, true
}

func parseCalendarOptions(c *gin.Context) (merge bool, includes, excludes matchGroup, ok bool) {
	merge, ok = parseMergeOption(c, c.Query("mrg"))
	if !ok {
		return false, nil, nil, false
	}

	includes, err := parseMatchers(c.QueryArray("inc"))
	if err != nil {
		err = fmt.Errorf("error parsing inc argument %q: %s", c.QueryArray("inc"), err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return false, nil, nil, false
	}
	if len(includes) == 0 {
		includes = append(includes, matcher{
			property:   ics.ComponentPropertySummary,
			expression: regexp.MustCompile(".*"),
		})
	}

	excludes, err = parseMatchers(c.QueryArray("exc"))
	if err != nil {
		err = fmt.Errorf("error parsing exc argument %q: %s", c.QueryArray("exc"), err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return false, nil, nil, false
	}

	return merge, includes, excludes, true
}
