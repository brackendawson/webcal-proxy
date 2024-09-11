package server_test

import (
	_ "embed"

	server "github.com/brackendawson/webcal-proxy"
)

var (
	//go:embed fixtures/calExample.ics
	calExample []byte
	//go:embed fixtures/calOnlyRotation.ics
	calOnlyRotation []byte
	//go:embed fixtures/calWithoutSecondary.ics
	calWithoutSecondary []byte
	//go:embed fixtures/calMay22NotRotation.ics
	calMay22NotRotation []byte
	//go:embed fixtures/calShuffled.ics
	calShuffled []byte
	//go:embed fixtures/calEventWithNoStart.ics
	calEventWithNoStart []byte
	//go:embed fixtures/calUnmerged.ics
	calUnmerged []byte
	//go:embed fixtures/calMerged.ics
	calMerged []byte

	month11september2024 = []server.Day{
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
)
