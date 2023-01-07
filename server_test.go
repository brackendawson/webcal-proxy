package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	ics "github.com/arran4/golang-ical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexFilter(t *testing.T) {
	for name, test := range map[string]struct {
		source        []byte
		tryRedirect   bool
		options       string
		allowLoopback bool
		wantSourceURL string
		want          []byte
		wantStatus    int
	}{
		"default": {
			source:        []byte(calExample),
			options:       "?cal=http://CALURL",
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calExample),
		},
		"no-cal": {
			source:        []byte(calExample),
			options:       "?not=right",
			allowLoopback: true,
			wantStatus:    400,
		},
		"includeRotation": {
			source:        []byte(calExample),
			options:       "?cal=http://CALURL&inc=SUMMARY=Rotation",
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calOnlyRotation),
		},
		"excludeSecondary": {
			source:        []byte(calExample),
			options:       "?cal=http://CALURL&exc=SUMMARY=Secondary",
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calWithoutSecondary),
		},
		"includeExclude": {
			source:        []byte(calExample),
			options:       `?cal=http://CALURL&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calMay22NotRotation),
		},
		"local": {
			source:     []byte(calExample),
			options:    `?cal=http://127.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"private": {
			source:     []byte(calExample),
			options:    `?cal=http://192.168.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"vpn": {
			source:     []byte(calExample),
			options:    `?cal=http://10.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"localhost": {
			source:     []byte(calExample),
			options:    `?cal=http://localhost:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"no-port-localhost": {
			source:     []byte(calExample),
			options:    `?cal=http://localhost&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"webcal": {
			source:        []byte(calExample),
			options:       `?cal=webcal://CALURL`,
			allowLoopback: true,
			wantStatus:    200,
			want:          []byte(calExample),
		},
		"ftp": {
			source:        []byte(calExample),
			options:       `?cal=ftp://CALURL`,
			allowLoopback: true,
			wantStatus:    400,
		},
		"redirect": {
			tryRedirect:   true,
			options:       `?cal=webcal://CALURL`,
			allowLoopback: true,
			wantStatus:    502,
		},
		"unresolvable": {
			options:    "?cal=webcal://not.a.domain",
			wantStatus: 502,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.tryRedirect {
					http.Redirect(w, r, "192.168.0.1", http.StatusMovedPermanently)
					return
				}
				w.Header().Set("Content-Type", "text/calendar")
				_, _ = w.Write(test.source)
			}))
			defer ts.Close()

			tsURL, err := url.Parse(ts.URL)
			require.NoError(t, err)
			url := "http://localhost" + strings.Replace(test.options, "CALURL", tsURL.Host, 1)
			t.Log(url)
			r := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			defer func(prev bool) { allowLoopback = prev }(allowLoopback)
			allowLoopback = test.allowLoopback

			s := Server{}
			s.HandleWebcal(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.want != nil {
				wantCal, err := ics.ParseCalendar(bytes.NewReader(test.want))
				require.NoError(t, err)
				assert.Equal(t, []byte(wantCal.Serialize()), w.Body.Bytes())
				assert.Equal(t, "text/calendar", w.Header().Get("Content-Type"))
				t.Logf("want:\n%s", wantCal.Serialize())
				t.Logf("got:\n%s", w.Body.String())
			}
		})
	}
}
