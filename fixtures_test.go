package server_test

import (
	"time"

	server "github.com/brackendawson/webcal-proxy"
)

func daysSeptember2024In(l *time.Location) []server.Day {
	return []server.Day{
		{Time: time.Date(2024, 8, 26, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 8, 27, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 8, 28, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 8, 29, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 8, 30, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 8, 31, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 1, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 2, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 3, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 4, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 5, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 6, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 7, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 8, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 9, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 10, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 11, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 12, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 13, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 14, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 15, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 16, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 17, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 18, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 19, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 20, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 21, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 22, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 23, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 24, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 25, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 26, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 27, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 28, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 29, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 9, 30, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 10, 1, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 10, 2, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 10, 3, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 10, 4, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 10, 5, 0, 0, 0, 0, l)},
		{Time: time.Date(2024, 10, 6, 0, 0, 0, 0, l)},
	}
}

var (
	locSydney  = must(time.LoadLocation("Australia/Sydney"))
	locNewYork = must(time.LoadLocation("America/New_York"))

	daysSept2024WithEvents = func() []server.Day {
		c := daysSeptember2024In(time.UTC)
		c[10+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC),
				EndTime:     time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC),
				Summary:     "Meeting",
				Location:    "Office",
				Description: "Take notes",
			},
		}
		c[11+5].Events = []server.Event{
			{
				StartTime: time.Date(2024, 9, 11, 11, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 9, 11, 12, 0, 0, 0, time.UTC),
				Summary:   "Picnic",
				Location:  "Park",
			},
		}
		c[2+30+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 10, 2, 0, 0, 0, 0, time.UTC),
				EndTime:     time.Date(2024, 10, 3, 0, 0, 0, 0, time.UTC),
				Summary:     "Barbie's birthday",
				Description: "bring cake",
			},
		}
		return c
	}()

	daysSept2024WithAllDayEvent = func() []server.Day {
		c := daysSeptember2024In(time.UTC)
		c[23+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 9, 23, 0, 0, 0, 0, time.UTC),
				EndTime:     time.Date(2024, 9, 24, 0, 0, 0, 0, time.UTC),
				Summary:     "Meeting",
				Location:    "Office",
				Description: "Talk",
			},
		}
		return c
	}()

	daysSept2024WithMultiDayEvent = func() []server.Day {
		c := daysSeptember2024In(time.UTC)
		c[22+5].Events = []server.Event{
			{
				StartTime: time.Date(2024, 9, 22, 11, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 9, 24, 11, 0, 0, 0, time.UTC),
				Summary:   "Festival",
			},
		}
		c[23+5].Events = []server.Event{
			{
				StartTime: time.Date(2024, 9, 22, 11, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 9, 24, 11, 0, 0, 0, time.UTC),
				Summary:   "Festival",
			},
		}
		c[24+5].Events = []server.Event{
			{
				StartTime: time.Date(2024, 9, 22, 11, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 9, 24, 11, 0, 0, 0, time.UTC),
				Summary:   "Festival",
			},
		}
		return c
	}()

	daysSept2024WithEventsSydney = func() []server.Day {
		c := daysSeptember2024In(locSydney)
		c[10+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC).In(locSydney),
				EndTime:     time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC).In(locSydney),
				Summary:     "Meeting",
				Location:    "Office",
				Description: "Take notes",
			},
		}
		c[11+5].Events = []server.Event{
			{
				StartTime: time.Date(2024, 9, 11, 11, 0, 0, 0, time.UTC).In(locSydney),
				EndTime:   time.Date(2024, 9, 11, 12, 0, 0, 0, time.UTC).In(locSydney),
				Summary:   "Picnic",
				Location:  "Park",
			},
		}
		c[2+30+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 10, 2, 0, 0, 0, 0, locSydney),
				EndTime:     time.Date(2024, 10, 3, 0, 0, 0, 0, locSydney),
				Summary:     "Barbie's birthday",
				Description: "bring cake",
			},
		}
		return c
	}()
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
