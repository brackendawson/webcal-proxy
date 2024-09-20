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

func appendDay(ctx context.Context, s []Day, target, today time.Time, downstream *ics.Calendar, days ...time.Time) []Day {
	for _, d := range days {
		newDay := Day{
			Number:  d.Day(),
			Weekday: strings.ToLower(d.Weekday().String()),
			Today:   d.Month() == today.Month() && d.Day() == today.Day(),
			Spill:   d.Month() != target.Month(),
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
				newEvent.StartTime = newEvent.StartTime.In(target.Location())
			}
			if newEvent.EndTime, err = event.GetEndAt(); err != nil {
				log(ctx).Warnf("Invalid event end time: %s", err) // TODO contribute a defined error here
				continue
			}
			if newEvent.EndTime.Location() != time.Local {
				newEvent.EndTime = newEvent.EndTime.In(target.Location())
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
	View
	CalendarView calendarView
	Target       time.Time
	Days         []Day
	Cache        *cache.Webcal
	URL          string
	Error        string
}

func newCalendar(ctx context.Context, view View, calendarView calendarView, target, today time.Time, downstream *ics.Calendar) Calendar {
	cal := Calendar{
		View:         view,
		Target:       target,
		CalendarView: calendarView,
	}

	float := target.AddDate(0, 0, -target.Day()+1)
	endOfMonth := float.AddDate(0, 1, 0)
	float = float.AddDate(0, 0, -mondayIndexWeekday(float.Weekday()))

	for float.Before(endOfMonth) {
		cal.Days = appendDay(ctx, cal.Days, target, today, downstream, float)
		float = float.AddDate(0, 0, 1)
	}
	for float.Weekday() != time.Monday {
		cal.Days = appendDay(ctx, cal.Days, target, today, downstream, float)
		float = float.AddDate(0, 0, 1)
	}

	return cal
}

func mondayIndexWeekday(d time.Weekday) int {
	return ((int(d)-1)%7 + 7) % 7
}
