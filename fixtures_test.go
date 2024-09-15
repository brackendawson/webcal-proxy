package server_test

import (
	"time"

	server "github.com/brackendawson/webcal-proxy"
)

var (
	month11Sept2024 = []server.Day{
		{Number: 26, Weekday: "monday", Spill: true},
		{Number: 27, Weekday: "tuesday", Spill: true},
		{Number: 28, Weekday: "wednesday", Spill: true},
		{Number: 29, Weekday: "thursday", Spill: true},
		{Number: 30, Weekday: "friday", Spill: true},
		{Number: 31, Weekday: "saturday", Spill: true},
		{Number: 1, Weekday: "sunday"},
		{Number: 2, Weekday: "monday"},
		{Number: 3, Weekday: "tuesday"},
		{Number: 4, Weekday: "wednesday"},
		{Number: 5, Weekday: "thursday"},
		{Number: 6, Weekday: "friday"},
		{Number: 7, Weekday: "saturday"},
		{Number: 8, Weekday: "sunday"},
		{Number: 9, Weekday: "monday"},
		{Number: 10, Weekday: "tuesday"},
		{Number: 11, Weekday: "wednesday", Today: true},
		{Number: 12, Weekday: "thursday"},
		{Number: 13, Weekday: "friday"},
		{Number: 14, Weekday: "saturday"},
		{Number: 15, Weekday: "sunday"},
		{Number: 16, Weekday: "monday"},
		{Number: 17, Weekday: "tuesday"},
		{Number: 18, Weekday: "wednesday"},
		{Number: 19, Weekday: "thursday"},
		{Number: 20, Weekday: "friday"},
		{Number: 21, Weekday: "saturday"},
		{Number: 22, Weekday: "sunday"},
		{Number: 23, Weekday: "monday"},
		{Number: 24, Weekday: "tuesday"},
		{Number: 25, Weekday: "wednesday"},
		{Number: 26, Weekday: "thursday"},
		{Number: 27, Weekday: "friday"},
		{Number: 28, Weekday: "saturday"},
		{Number: 29, Weekday: "sunday"},
		{Number: 30, Weekday: "monday"},
		{Number: 1, Weekday: "tuesday", Spill: true},
		{Number: 2, Weekday: "wednesday", Spill: true},
		{Number: 3, Weekday: "thursday", Spill: true},
		{Number: 4, Weekday: "friday", Spill: true},
		{Number: 5, Weekday: "saturday", Spill: true},
		{Number: 6, Weekday: "sunday", Spill: true},
	}
	month11Sept2024Sydney = []server.Day{
		{Number: 26, Weekday: "monday", Spill: true},
		{Number: 27, Weekday: "tuesday", Spill: true},
		{Number: 28, Weekday: "wednesday", Spill: true},
		{Number: 29, Weekday: "thursday", Spill: true},
		{Number: 30, Weekday: "friday", Spill: true},
		{Number: 31, Weekday: "saturday", Spill: true},
		{Number: 1, Weekday: "sunday"},
		{Number: 2, Weekday: "monday"},
		{Number: 3, Weekday: "tuesday"},
		{Number: 4, Weekday: "wednesday"},
		{Number: 5, Weekday: "thursday"},
		{Number: 6, Weekday: "friday"},
		{Number: 7, Weekday: "saturday"},
		{Number: 8, Weekday: "sunday"},
		{Number: 9, Weekday: "monday"},
		{Number: 10, Weekday: "tuesday"},
		{Number: 11, Weekday: "wednesday"},
		{Number: 12, Weekday: "thursday", Today: true},
		{Number: 13, Weekday: "friday"},
		{Number: 14, Weekday: "saturday"},
		{Number: 15, Weekday: "sunday"},
		{Number: 16, Weekday: "monday"},
		{Number: 17, Weekday: "tuesday"},
		{Number: 18, Weekday: "wednesday"},
		{Number: 19, Weekday: "thursday"},
		{Number: 20, Weekday: "friday"},
		{Number: 21, Weekday: "saturday"},
		{Number: 22, Weekday: "sunday"},
		{Number: 23, Weekday: "monday"},
		{Number: 24, Weekday: "tuesday"},
		{Number: 25, Weekday: "wednesday"},
		{Number: 26, Weekday: "thursday"},
		{Number: 27, Weekday: "friday"},
		{Number: 28, Weekday: "saturday"},
		{Number: 29, Weekday: "sunday"},
		{Number: 30, Weekday: "monday"},
		{Number: 1, Weekday: "tuesday", Spill: true},
		{Number: 2, Weekday: "wednesday", Spill: true},
		{Number: 3, Weekday: "thursday", Spill: true},
		{Number: 4, Weekday: "friday", Spill: true},
		{Number: 5, Weekday: "saturday", Spill: true},
		{Number: 6, Weekday: "sunday", Spill: true},
	}

	month11Sept2024WithEvents = func() []server.Day {
		c := make([]server.Day, len(month11Sept2024))
		copy(c, month11Sept2024)
		c[10+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC).In(time.UTC),
				EndTime:     time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC).In(time.UTC),
				Summary:     "Meeting",
				Location:    "Office",
				Description: "Take notes",
			},
		}
		c[11+5].Events = []server.Event{
			{
				StartTime: time.Date(2024, 9, 11, 11, 0, 0, 0, time.UTC).In(time.UTC),
				EndTime:   time.Date(2024, 9, 11, 12, 0, 0, 0, time.UTC).In(time.UTC),
				Summary:   "Picnic",
				Location:  "Park",
			},
		}
		c[2+30+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 10, 2, 0, 0, 0, 0, time.Local),
				EndTime:     time.Date(2024, 10, 3, 0, 0, 0, 0, time.Local),
				Summary:     "Barbie's birthday",
				Description: "bring cake",
			},
		}
		return c
	}()
	month11Sept2024WithEventsSydney = func() []server.Day {
		tzSydney, err := time.LoadLocation("Australia/Sydney")
		if err != nil {
			panic(err)
		}
		c := make([]server.Day, len(month11Sept2024))
		copy(c, month11Sept2024Sydney)
		c[10+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC).In(tzSydney),
				EndTime:     time.Date(2024, 9, 10, 11, 0, 0, 0, time.UTC).In(tzSydney),
				Summary:     "Meeting",
				Location:    "Office",
				Description: "Take notes",
			},
		}
		c[11+5].Events = []server.Event{
			{
				StartTime: time.Date(2024, 9, 11, 11, 0, 0, 0, time.UTC).In(tzSydney),
				EndTime:   time.Date(2024, 9, 11, 12, 0, 0, 0, time.UTC).In(tzSydney),
				Summary:   "Picnic",
				Location:  "Park",
			},
		}
		c[2+30+5].Events = []server.Event{
			{
				StartTime:   time.Date(2024, 10, 2, 0, 0, 0, 0, time.Local),
				EndTime:     time.Date(2024, 10, 3, 0, 0, 0, 0, time.Local),
				Summary:     "Barbie's birthday",
				Description: "bring cake",
			},
		}
		return c
	}()
)
