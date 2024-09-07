package server

import (
	"errors"
	"net/url"
	"time"

	ics "github.com/arran4/golang-ical"
)

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
