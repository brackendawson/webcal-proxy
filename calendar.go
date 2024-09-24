package server

import (
	"context"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/brackendawson/webcal-proxy/cache"
)

// Event is not an exhaustive view of event components
type Event struct {
	StartTime, EndTime             time.Time
	Summary, Location, Description string
}

type Day struct {
	time.Time
	// Events are the events in this day, or nil if the day has no events, or if
	// []Day was made with a nil downstream.
	Events []Event
}

func appendDay(ctx context.Context, s []Day, target, today time.Time, downstream *ics.Calendar, days ...time.Time) []Day {
	for _, d := range days {
		thisDay := Day{
			Time: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()),
		}

		if downstream == nil {
			s = append(s, thisDay)
			continue
		}

		for _, component := range downstream.Components {
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
			// Date only events (no time) are parsed to midnight on time.Local,
			// this must be normalised to the target time zone for inequalities
			// to work.
			if newEvent.StartTime.Location() == time.Local {
				newEvent.StartTime = setLocation(newEvent.StartTime, target.Location())
			}
			newEvent.StartTime = newEvent.StartTime.In(target.Location())
			if newEvent.StartTime.After(thisDay.AddDate(0, 0, 1)) ||
				newEvent.StartTime.Equal(thisDay.AddDate(0, 0, 1)) {
				continue
			}

			if newEvent.EndTime, err = event.GetEndAt(); err != nil {
				log(ctx).Warnf("Invalid event end time: %s", err) // TODO contribute a defined error here
				continue
			}
			if newEvent.EndTime.Location() == time.Local {
				newEvent.EndTime = setLocation(newEvent.EndTime, target.Location())
			}
			newEvent.EndTime = newEvent.EndTime.In(target.Location())
			if !newEvent.EndTime.IsZero() &&
				newEvent.EndTime.Before(thisDay.Time) ||
				newEvent.EndTime.Equal(thisDay.Time) {
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

			thisDay.Events = append(thisDay.Events, newEvent)
		}

		s = append(s, thisDay)
	}
	return s
}

// SameDate returns true if as has the same calendar date as c, regardless of
// location.
func (d Day) SameDate(as time.Time) bool {
	return d.Day() == as.Day() &&
		d.Month() == as.Month() &&
		d.Year() == as.Year()
}

// setLocation changes a time.Time's location without changing the clock time.
func setLocation(t time.Time, l *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(),
		t.Second(), t.Nanosecond(), l)
}

type Month struct {
	View
	// Target is the date the user wishes to view, it may have a day, time, or
	// location set.
	Target time.Time
	// Now is the user's current time, it may have a day, time, or location set.
	Now time.Time
	// Days are the days in the Target Month plus the days before and after the
	// target month to fill incomplete leading and trailing weeks.
	Days []Day
	// Cache is always the unfiltered upstream ICS or nil.
	Cache *cache.Webcal
	// URL is the new webcal:// link for the User.
	URL string
	// Error is the error to show to the user or empty string.
	Error string
}

func newMonth(ctx context.Context, view View, target, today time.Time, downstream *ics.Calendar) Month {
	cal := Month{
		View:   view,
		Target: target,
		Now:    today,
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
