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
)
