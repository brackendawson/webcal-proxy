package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ics "github.com/arran4/golang-ical"
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
	r.GET("/", s.HandleWebcal)
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

func (s *Server) parseCalendarURL(addr string) (string, error) {
	if addr == "" {
		return "", errors.New("missing query paramater: cal")
	}

	addrString, err := url.QueryUnescape(addr)
	if err != nil {
		return "", errors.New("invalid calendar url")
	}

	addrURL, err := url.Parse(addrString)
	if err != nil {
		return "", errors.New("invalid calendar url")
	}

	if addrURL.Scheme == "webcal" {
		addrURL.Scheme = "http"
	}
	if addrURL.Scheme != "http" && addrURL.Scheme != "https" {
		return "", errors.New("invalid calendar url")
	}

	return addrURL.String(), nil
}

type matchGroup []matcher

func (m matchGroup) matches(event *ics.VEvent) bool {
	for _, matcher := range m {
		if matcher.regx.Match([]byte(event.GetProperty(matcher.property).Value)) {
			return true
		}
	}
	return false
}

type matcher struct {
	property ics.ComponentProperty
	regx     *regexp.Regexp
}

func parseMatchers(m []string) (matchGroup, error) {
	matches := make(matchGroup, 0, len(m))
	for _, matchOpt := range m {
		parts := strings.Split(matchOpt, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid match paramater: %s, should be <FIELD>=<regexp>", matchOpt)
		}
		regx, err := regexp.Compile(parts[1])
		if err != nil {
			return nil, fmt.Errorf("bad regexp in match paramater '%s': %s", matchOpt, err)
		}
		matches = append(matches, matcher{
			property: ics.ComponentProperty(parts[0]),
			regx:     regx,
		})
	}
	return matches, nil
}

// mergeEvents will perform the merge algorithm on a slice of events sorted by
// start time.
func mergeEvents(events []*ics.VEvent) ([]*ics.VEvent, error) {
	var (
		newEvents   []*ics.VEvent
		lastEndTime time.Time
	)

	for _, event := range events {
		startTime, err := event.GetStartAt()
		if err != nil {
			return nil, errors.New("event has no start time")
		}
		endTime, err := event.GetEndAt()
		if err != nil {
			return nil, errors.New("event has no end time")
		}

		if len(newEvents) == 0 || !startTime.Before(lastEndTime) {
			lastEndTime = endTime
			newEvents = append(newEvents, event)
			continue
		}

		lastEvent := newEvents[len(newEvents)-1]

		lastSummary := lastEvent.GetProperty(ics.ComponentPropertySummary)
		newSummary := ""
		if lastSummary != nil {
			newSummary = lastSummary.Value
		}
		summary := event.GetProperty(ics.ComponentPropertySummary)
		if summary != nil {
			newSummary += " + "
			newSummary += summary.Value
		}
		lastEvent.SetSummary(newSummary)

		lastDescription := lastEvent.GetProperty(ics.ComponentPropertyDescription)
		newDescription := ""
		if lastDescription != nil {
			newDescription = lastDescription.Value
		}
		description := event.GetProperty(ics.ComponentPropertyDescription)
		if description != nil {
			newDescription += "\n\n---\n"
			if summary != nil {
				newDescription += summary.Value + "\n"
			}
			newDescription += "\n"
			newDescription += description.Value
		}
		if newDescription != "" {
			lastEvent.SetProperty(ics.ComponentPropertyDescription, newDescription)
		}

		if endTime.After(lastEndTime) {
			var props []ics.PropertyParameter
			for k, v := range event.GetProperty(ics.ComponentPropertyDtEnd).ICalParameters {
				props = append(props, &ics.KeyValues{
					Key:   k,
					Value: v,
				})
			}
			lastEvent.SetProperty(ics.ComponentPropertyDtEnd, event.GetProperty(ics.ComponentPropertyDtEnd).Value, props...)
			lastEndTime = endTime
		}
	}

	return newEvents, nil
}
