package server

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/gin-gonic/gin"
)

type calendarView string

const (
	ViewMonth calendarView = "month"
)

// Event is not an exhaustive view of event components
type Event struct {
	StartTime, EndTime             time.Time
	Summary, Location, Description string
}

type Day struct {
	Number  int
	Weekday string
	Today   bool
	Spill   bool
	Events  []Event
}

func appendDay(c *gin.Context, s []Day, focus time.Time, downstream *ics.Calendar, days ...time.Time) []Day {
	for _, d := range days {
		newDay := Day{
			Number:  d.Day(),
			Weekday: strings.ToLower(d.Weekday().String()),
			Today:   d.Month() == focus.Month() && d.Day() == focus.Day(),
			Spill:   d.Month() != focus.Month(),
		}

		if downstream == nil {
			s = append(s, newDay)
			continue
		}

		for _, component := range downstream.Components { // TODO events are sorted by start time, we can avoid the O(n^2)
			event, ok := component.(*ics.VEvent)
			if !ok {
				continue
			}

			var (
				newEvent Event
				err      error
			)
			if newEvent.StartTime, err = event.GetStartAt(); err != nil {
				log(c).Warnf("Invalid event start time: %s", err)
				continue
			}
			if newEvent.EndTime, err = event.GetEndAt(); err != nil {
				log(c).Warnf("Invalid event end time: %s", err) // TODO contribute a defined error here
				continue
			}

			if newEvent.StartTime.Year() != d.Year() ||
				newEvent.StartTime.Month() != d.Month() ||
				newEvent.StartTime.Day() != d.Day() { // TODO multi day event, use inequalities
				continue
			}
			if summary := event.GetProperty(ics.ComponentPropertySummary); summary != nil {
				newEvent.Summary = summary.Value
			}
			if location := event.GetProperty(ics.ComponentPropertyLocation); location != nil {
				newEvent.Location = location.Value
			}
			if description := event.GetProperty(ics.ComponentPropertyDescription); description != nil {
				newEvent.Description = description.Value
			}

			newDay.Events = append(newDay.Events, newEvent)
		}

		s = append(s, newDay)
	}
	return s
}

type Calendar struct {
	View  calendarView
	Title string
	Days  []Day
	Cache *Cache
	Error string
}

func newCalendar(c *gin.Context, view calendarView, focus time.Time, downstream *ics.Calendar) Calendar {
	// TODO allow moving the focus date, need to think about the today date
	cal := Calendar{
		View: view,
	}

	cal.Title = focus.Format("January 2006")

	float := focus.AddDate(0, 0, -focus.Day()+1)
	float = float.AddDate(0, 0, -mondayIndexWeekday(float.Weekday()))

	for float.Month() <= focus.Month() {
		cal.Days = appendDay(c, cal.Days, focus, downstream, float)
		float = float.AddDate(0, 0, 1)
	}
	for float.Weekday() != time.Monday {
		cal.Days = appendDay(c, cal.Days, focus, downstream, float)
		float = float.AddDate(0, 0, 1)
	}

	return cal
}

func mondayIndexWeekday(d time.Weekday) int {
	return ((int(d)-1)%7 + 7) % 7
}

func parseURLScheme(c *gin.Context, addr string) (string, error) {
	addrURL, err := url.Parse(addr)
	if err != nil {
		log(c).Warnf("invalid calendar url: %s", err)
		return "", newErrorWithMessage(
			http.StatusBadRequest,
			"Bad url. Include a protocol, host, and path, eg: webcal://example.com/events",
		)
	}

	if addrURL.Scheme == "webcal" {
		addrURL.Scheme = "http"
	}

	if !slices.Contains([]string{"http", "https"}, addrURL.Scheme) {
		return "", newErrorWithMessage(
			http.StatusBadRequest,
			"Unsupported protocol scheme, url should be webcal, https, or http.",
		)
	}

	return addrURL.String(), nil
}

// mergeEvents will perform the merge algorithm on a slice of events sorted by
// start time.
func mergeEvents(events []*ics.VEvent) []*ics.VEvent {
	var (
		newEvents   []*ics.VEvent
		lastEndTime time.Time
	)

	for _, event := range events {
		startTime, _ := event.GetStartAt()
		endTime, _ := event.GetEndAt()
		if endTime.Before(startTime) {
			endTime = startTime
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

	return newEvents
}
