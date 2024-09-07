package server

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	ics "github.com/arran4/golang-ical"
	"github.com/brackendawson/webcal-proxy/assets"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

const (
	defaultMaxConns = 8
)

type Server struct {
	// MaxConns is the maximum concurrent upstream connections the server can
	// make. It cannot be changed after the first upstream connection is made.
	// If set to 0 then 8 connections are allowed.
	MaxConns   int
	semaphore  chan struct{}
	clientOnce sync.Once
}

func New(r *gin.Engine) *Server {
	s := &Server{}
	r.Use(requestid.New())
	r.Use(logging)
	r.SetHTMLTemplate(assets.Templates())
	r.GET("/", s.HandleWebcal)
	r.StaticFS("/assets", http.FS(assets.Assets))
	return s
}

func (s *Server) clientInit() {
	maxConns := s.MaxConns
	if maxConns < 1 {
		maxConns = defaultMaxConns
	}
	s.semaphore = make(chan struct{}, maxConns)
}

// fetch fetches the given url from the given IP address
func (s *Server) fetch(url string) (*ics.Calendar, error) {
	s.clientOnce.Do(s.clientInit)

	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	upstream, err := client.Get(url)
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
	if isBrowser(c.GetHeader("Accept")) {
		s.HandleIndex(c)
		return
	}

	upstreamURLString := c.Query("cal")
	upstreamURL, err := s.parseCalendarURL(upstreamURLString)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error validating calendar URL: %s", err))
		return
	}

	includes, err := parseMatchers(c.QueryArray("inc"))
	if err != nil {
		c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error parsing inc argument %q: %s", c.QueryArray("inc"), err))
		return
	}
	if len(includes) == 0 {
		includes = append(includes, matcher{
			property: ics.ComponentPropertySummary,
			regx:     regexp.MustCompile(".*"),
		})
	}
	excludes, err := parseMatchers(c.QueryArray("exc"))
	if err != nil {
		c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error parsing exc argument %q: %s", c.QueryArray("exc"), err))
		return
	}

	var merge bool
	if mergeArg := c.Query("mrg"); mergeArg != "" {
		merge, err = strconv.ParseBool(mergeArg)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("error parsing mrg argument: %s", err))
			return
		}
	}

	upstream, err := s.fetch(upstreamURL)
	if err != nil {
		c.AbortWithError(http.StatusBadGateway,
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
		c.AbortWithError(http.StatusBadRequest,
			fmt.Errorf("error sorting events: %s", err))
		return
	}

	if merge {
		events, err = mergeEvents(events)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest,
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

func isBrowser(accept string) bool {
	for _, mediaType := range strings.Split(accept, ",") {
		if mediaType == "text/html" {
			return true
		}
	}
	return false
}
