package server

import (
	"sort"
	"time"

	ics "github.com/arran4/golang-ical"
)

func getDownstreamCalendar(upstream *ics.Calendar, opts calenderOptions) *ics.Calendar {
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
		includes := opts.includes
		if len(opts.includes) == 0 {
			includes = defaultMatches
		}

		if includes.matches(event) && !opts.excludes.matches(event) {
			events = append(events, event)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		startI, _ := events[i].GetStartAt()
		startJ, _ := events[j].GetStartAt()
		return startI.Before(startJ)
	})

	if opts.merge {
		events = mergeEvents(events)
	}

	for _, event := range events {
		downstream.AddVEvent(event)
	}

	return downstream
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
