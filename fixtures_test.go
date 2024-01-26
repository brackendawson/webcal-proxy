package server

import (
	_ "embed"
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
)
