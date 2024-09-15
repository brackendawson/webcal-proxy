package fixtures

import (
	_ "embed"
)

var (
	//go:embed calExample.ics
	CalExample []byte
	//go:embed calOnlyRotation.ics
	CalOnlyRotation []byte
	//go:embed calWithoutSecondary.ics
	CalWithoutSecondary []byte
	//go:embed calMay22NotRotation.ics
	CalMay22NotRotation []byte
	//go:embed calShuffled.ics
	CalShuffled []byte
	//go:embed calEventWithNoStart.ics
	CalEventWithNoStart []byte
	//go:embed calEventWithNoStartSorted.ics
	CalEventWithNoStartSorted []byte
	//go:embed calUnMerged.ics
	CalUnMerged []byte
	//go:embed calMerged.ics
	CalMerged []byte
	//go:embed events11Sept2024.ics
	Events11Sept2024 []byte
	//go:embed emptyCalendar.ics
	EmptyCalendar []byte
)
