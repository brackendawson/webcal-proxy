package server

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	ics "github.com/arran4/golang-ical"
)

type Server struct {
	Client     *http.Client
	clientOnce sync.Once
}

func (s *Server) clientInit() {
	if s.Client == nil {
		s.Client = &http.Client{
			Timeout: time.Minute,
		}
	}
}

func (s *Server) fetch(url string) (*ics.Calendar, error) {
	s.clientOnce.Do(s.clientInit)

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
	// TODO handle POST et al

	upstreamURL := r.URL.Query().Get("cal")
	if upstreamURL == "" {
		http.Error(w, "Missing query paramater: cal", http.StatusBadRequest)
		return
	}

	upstream, err := s.fetch(upstreamURL)
	if err != nil {
		http.Error(w, "Failed to fetch calendar: "+err.Error(), http.StatusBadGateway)
		return
	}

	downstream := ics.NewCalendar()
	downstream.CalendarProperties = upstream.CalendarProperties

	matchOpts := r.URL.Query()["match"]
	type matcher struct {
		property ics.ComponentProperty
		regx     *regexp.Regexp
	}
	matches := make([]matcher, 0, len(matchOpts))
	for _, match := range matchOpts {
		parts := strings.Split(match, "=")
		if len(parts) != 2 {
			http.Error(w, fmt.Sprintf("invalid match paramater: %s, should be <FIELD>=<regexp>", match), http.StatusBadRequest)
			return
		}
		regx, err := regexp.Compile(parts[1])
		if err != nil {
			http.Error(w, fmt.Sprintf("bad regexp in match paramater '%s': %s", match, err), http.StatusBadRequest)
			return
		}
		matches = append(matches, matcher{
			property: ics.ComponentProperty(parts[0]),
			regx:     regx,
		})
	}

	for _, event := range upstream.Events() {
		for _, match := range matches {
			if match.regx.Match([]byte(event.GetProperty(match.property).Value)) {
				downstream.AddVEvent(event)
			}
		}
	}

	w.Header().Set("Content-Type", r.Header.Get("Contnet-Type"))
	downstream.SerializeTo(w)
}
