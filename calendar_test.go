package server_test

import (
	"testing"
	"time"

	server "github.com/brackendawson/webcal-proxy"
	"github.com/stretchr/testify/require"
)

func TestDaySameDate(t *testing.T) {
	for name, test := range map[string]struct {
		a, b     time.Time
		expected bool
	}{
		"identical": {
			a:        time.Date(2024, 9, 24, 00, 00, 0, 0, time.UTC),
			b:        time.Date(2024, 9, 24, 00, 00, 0, 0, time.UTC),
			expected: true,
		},
		"close_enough": {
			a:        time.Date(2024, 9, 24, 00, 00, 0, 0, time.UTC),
			b:        time.Date(2024, 9, 24, 23, 59, 59, 999_999_999, time.UTC),
			expected: true,
		},
		"different_locations_same_date": {
			a:        time.Date(2024, 9, 24, 23, 00, 0, 0, time.UTC),
			b:        time.Date(2024, 9, 24, 23, 00, 0, 0, locSydney),
			expected: true,
		},
		"different_day": {
			a:        time.Date(2024, 9, 24, 00, 00, 0, 0, time.UTC),
			b:        time.Date(2024, 9, 25, 00, 00, 0, 0, time.UTC),
			expected: false,
		},
		"different_month": {
			a:        time.Date(2024, 9, 24, 00, 00, 0, 0, time.UTC),
			b:        time.Date(2024, 10, 24, 00, 00, 0, 0, time.UTC),
			expected: false,
		},
		"different_year": {
			a:        time.Date(2024, 9, 24, 00, 00, 0, 0, time.UTC),
			b:        time.Date(2025, 9, 24, 00, 00, 0, 0, time.UTC),
			expected: false,
		},
		"different_locations_different_date": {
			// but would be the same date in UTC
			a:        time.Date(2024, 9, 24, 23, 00, 0, 0, time.UTC),
			b:        time.Date(2025, 9, 25, 01, 00, 0, 0, locSydney),
			expected: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, server.Day{Time: test.a}.SameDate(test.b))
		})
	}
}
