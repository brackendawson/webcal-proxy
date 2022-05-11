package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ics "github.com/arran4/golang-ical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexFilter(t *testing.T) {
	for name, test := range map[string]struct {
		source        []byte
		options       string
		allowLoopback bool
		wantSourceURL string
		want          []byte
		wantStatus    int
	}{
		"default": {
			source:        []byte(calExample),
			options:       "?cal=CALURL",
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calExample),
		},
		"includeRotation": {
			source:        []byte(calExample),
			options:       "?cal=CALURL&inc=SUMMARY=Rotation",
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calOnlyRotation),
		},
		"excludeSecondary": {
			source:        []byte(calExample),
			options:       "?cal=CALURL&exc=SUMMARY=Secondary",
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calWithoutSecondary),
		},
		"includeExclude": {
			source:        []byte(calExample),
			options:       `?cal=CALURL&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calMay22NotRotation),
		},
		"local": {
			source:     []byte(calExample),
			options:    `?cal=http://127.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 400,
		},
		"private": {
			source:     []byte(calExample),
			options:    `?cal=http://192.168.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 400,
		},
		"vpn": {
			source:     []byte(calExample),
			options:    `?cal=http://10.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 400,
		},
		"localhost": {
			source:     []byte(calExample),
			options:    `?cal=http://localhost:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 400,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/calendar")
				w.Write(test.source)
			}))
			defer ts.Close()

			url := "http://localhost" + strings.Replace(test.options, "CALURL", ts.URL, 1)
			t.Log(url)
			r := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			s := Server{
				allowLoopback: test.allowLoopback,
			}
			s.HandleWebcal(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.want != nil {
				wantCal, err := ics.ParseCalendar(bytes.NewReader(test.want))
				require.NoError(t, err)
				assert.Equal(t, []byte(wantCal.Serialize()), w.Body.Bytes())
				t.Logf("want:\n%s", wantCal.Serialize())
				t.Logf("got:\n%s", string(w.Body.Bytes()))
			}
		})
	}
}
