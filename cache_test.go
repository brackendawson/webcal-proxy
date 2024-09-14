package server_test

import (
	"bytes"
	"testing"

	ics "github.com/arran4/golang-ical"
	server "github.com/brackendawson/webcal-proxy"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	t.Parallel()

	initial := server.Cache{
		URL: "webcal://alpaca-racing.com/schedule",
		Calendar: func() *ics.Calendar {
			c, err := ics.ParseCalendar(bytes.NewReader(calExample))
			require.NoError(t, err)
			return c
		}(),
	}
	encoded, err := initial.Encode()
	require.NoError(t, err)

	t.Log(string(encoded))
	require.Less(t, len(encoded), len(calExample))

	decoded, err := server.ParseCache(encoded)
	require.NoError(t, err)
	require.Equal(t, initial, decoded)
}

func TestDecodeCacheErrors(t *testing.T) {
	for name, test := range map[string]struct {
		input        string
		requireError func(require.TestingT, error, ...any)
	}{
		"bad_base64":      {"I'm not base64", errorContains("error decoding cache: ")},
		"bad_gzip_header": {"SSdtIG5vdCBnemlw", errorContains("error decoding cache headers: ")},
		"bad_gzip_body":   {"H4sIACSSdtIG5vdCBnemlwetyEwu5gIAtDzF3gwAAAA=", errorContains("error decoding cache body: ")},
		"bad_ics":         {"H4sIACmx5GYAA/NUz1XIyy9RyEwu5gIAtDzF3gwAAAA=", errorContains("error parsing cached calendar: ")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := server.ParseCache(test.input)
			test.requireError(t, err)
		})
	}
}

func errorContains(contains string) func(require.TestingT, error, ...any) {
	return func(t require.TestingT, err error, msgAndArgs ...any) {
		require.ErrorContains(t, err, contains, msgAndArgs...)
	}
}
