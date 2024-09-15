package server

import (
	"context"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/brackendawson/webcal-proxy/cache"
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

func appendDay(ctx context.Context, s []Day, focus time.Time, downstream *ics.Calendar, days ...time.Time) []Day {
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
				log(ctx).Warnf("Invalid event start time: %s", err)
				continue
			}
			// Date only (no zone) events are parsed to midnight on time.Local,
			// don't convert this to user's time zone.
			if newEvent.StartTime.Location() != time.Local {
				newEvent.StartTime = newEvent.StartTime.In(focus.Location())
			}
			if newEvent.EndTime, err = event.GetEndAt(); err != nil {
				log(ctx).Warnf("Invalid event end time: %s", err) // TODO contribute a defined error here
				continue
			}
			if newEvent.EndTime.Location() != time.Local {
				newEvent.EndTime = newEvent.EndTime.In(focus.Location())
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
	Cache *cache.Webcal
	URL   string
	Error string
}

func newCalendar(ctx context.Context, view calendarView, focus time.Time, downstream *ics.Calendar) Calendar {
	// TODO allow moving the focus date, need to think about the today date
	cal := Calendar{
		View: view,
	}

	cal.Title = focus.Format("January 2006")

	float := focus.AddDate(0, 0, -focus.Day()+1)
	float = float.AddDate(0, 0, -mondayIndexWeekday(float.Weekday()))

	for float.Month() <= focus.Month() {
		cal.Days = appendDay(ctx, cal.Days, focus, downstream, float)
		float = float.AddDate(0, 0, 1)
	}
	for float.Weekday() != time.Monday {
		cal.Days = appendDay(ctx, cal.Days, focus, downstream, float)
		float = float.AddDate(0, 0, 1)
	}

	return cal
}

func mondayIndexWeekday(d time.Weekday) int {
	return ((int(d)-1)%7 + 7) % 7
}
