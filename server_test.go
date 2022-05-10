package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	ics "github.com/arran4/golang-ical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexFilter(t *testing.T) {
	for name, test := range map[string]struct {
		source        []byte
		options       string
		wantSourceURL string
		want          []byte
		wantStatus    int
	}{
		"default": {
			source:     []byte(calExample),
			options:    "",
			wantStatus: 200,
			want:       []byte(calExample),
		},
		"includeRotation": {
			source:     []byte(calExample),
			options:    "&inc=SUMMARY=Rotation",
			wantStatus: 200,
			want:       []byte(calOnlyRotation),
		},
		"excludeSecondary": {
			source:     []byte(calExample),
			options:    "&exc=SUMMARY=Secondary",
			wantStatus: 200,
			want:       []byte(calWithoutSecondary),
		},
		"includeExclude": {
			source:     []byte(calExample),
			options:    `&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 200,
			want:       []byte(calMay22NotRotation),
		},
	} {
		t.Run(name, func(t *testing.T) {
			src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/calendar")
				w.Write(test.source)
			}))
			defer src.Close()

			t.Log(src.URL)
			r := httptest.NewRequest("GET", "http://localhost?cal="+src.URL+test.options, nil)
			w := httptest.NewRecorder()

			s := Server{}
			s.HandleWebcal(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			wantCal, err := ics.ParseCalendar(bytes.NewReader(test.want))
			require.NoError(t, err)
			assert.Equal(t, []byte(wantCal.Serialize()), w.Body.Bytes())
			t.Logf("want:\n%s", wantCal.Serialize())
			t.Logf("got:\n%s", string(w.Body.Bytes()))
		})
	}
}
