package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	defaultMaxConns = 8
)

type Server struct {
	Client     *http.Client
	MaxConns   int
	semaphore  chan struct{}
	clientOnce sync.Once

	allowLoopback bool
}

func (s *Server) clientInit() {
	if s.Client == nil {
		s.Client = &http.Client{
			Timeout: time.Minute,
		}
	}

	maxConns := s.MaxConns
	if maxConns < 1 {
		maxConns = defaultMaxConns
	}
	s.semaphore = make(chan struct{}, maxConns)
}

func (s *Server) fetch(url string) (*ics.Calendar, error) {
	s.clientOnce.Do(s.clientInit)

	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	upstream, err := s.Client.Get(url)
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

func (s *Server) HandleWebcal(w http.ResponseWriter, r *http.Request) {
	log := logrus.WithField("request", uuid.New())

	if r.Method != http.MethodGet {
		log.Errorf("Reveived %s request", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	upstreamURLString := r.URL.Query().Get("cal")
	if upstreamURLString == "" {
		log.Error("Missing cal param")
		http.Error(w, "Missing query paramater: cal", http.StatusBadRequest)
		return
	}

	upstreamURLString, err := url.QueryUnescape(upstreamURLString)
	if err != nil {
		log.Error("Invalid cal param: ", upstreamURLString)
		http.Error(w, "Invalid calendar url", http.StatusBadRequest)
		return
	}

	upstreamURL, err := url.Parse(upstreamURLString)
	if err != nil {
		log.Error("Invalid url: ", err)
		http.Error(w, "Invalid calendar url", http.StatusBadRequest)
		return
	}

	addrs, err := net.LookupHost(upstreamURL.Hostname())
	if err != nil {
		log.Error("Failed to lookup host: ", err)
		http.Error(w, "Failed to fetch calendar: "+err.Error(), http.StatusBadGateway)
		return
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		switch {
		case s.allowLoopback && ip.IsLoopback():
		case !ip.IsGlobalUnicast() || ip.IsPrivate():
			log.Error("Denied access to private address: ", ip.String())
			http.Error(w, "Invalid calendar url", http.StatusBadRequest)
			return
		}
	}

	log.Info("Fetching: ", upstreamURL.String())

	if upstreamURL.Scheme == "webcal" {
		upstreamURL.Scheme = "http"
	}
	if upstreamURL.Scheme != "http" && upstreamURL.Scheme != "https" {
		log.Error("Wrong protocol scheme for calendar: ", upstreamURL.Scheme)
		http.Error(w, "Invalid calendar url", http.StatusBadRequest)
		return
	}

	upstream, err := s.fetch(upstreamURL.String())
	if err != nil {
		log.Errorf("Failed to fetch %q: %s", upstreamURL.String(), err)
		http.Error(w, "Failed to fetch calendar: "+err.Error(), http.StatusBadGateway)
		return
	}

	downstream := ics.NewCalendar()
	downstream.CalendarProperties = upstream.CalendarProperties

	includes, err := parseMatchers(r.URL.Query()["inc"])
	if err != nil {
		log.Errorf("Bad inc %q: %s", r.URL.Query()["inc"], err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(includes) == 0 {
		includes = append(includes, matcher{property: ics.ComponentPropertySummary, regx: regexp.MustCompile(".*")})
	}
	excludes, err := parseMatchers(r.URL.Query()["exc"])
	if err != nil {
		log.Errorf("Bad exc %q: %s", r.URL.Query()["exc"], err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, event := range upstream.Events() {
		if includes.matches(event) && !excludes.matches(event) {
			downstream.AddVEvent(event)
		}
	}

	w.Header().Set("Content-Type", r.Header.Get("Contnet-Type"))
	downstream.SerializeTo(w)

	log.Infof("Served %d/%d events", len(downstream.Events()), len(upstream.Events()))
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
