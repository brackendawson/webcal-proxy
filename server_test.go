package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	ics "github.com/arran4/golang-ical"
	"github.com/brackendawson/webcal-proxy/assets"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRegexFilter(t *testing.T) {
	for name, test := range map[string]struct {
		source          []byte
		tryRedirect     bool
		options         string
		headers         map[string]string
		allowLoopback   bool
		wantSourceURL   string
		wantCal         []byte
		wantBody        []byte
		wantHTTPRenders map[string]any
		wantStatus      int
	}{
		"default": {
			source:        calExample,
			options:       "?cal=http://CALURL",
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calExample,
		},
		"no-cal": {
			source:        calExample,
			options:       "?not=right",
			allowLoopback: true,
			wantStatus:    400,
		},
		"includeRotation": {
			source:        calExample,
			options:       "?cal=http://CALURL&inc=SUMMARY=Rotation",
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calOnlyRotation,
		},
		"excludeSecondary": {
			source:        calExample,
			options:       "?cal=http://CALURL&exc=SUMMARY=Secondary",
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calWithoutSecondary,
		},
		"includeExclude": {
			source:        calExample,
			options:       `?cal=http://CALURL&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calMay22NotRotation,
		},
		"local": {
			source:     calExample,
			options:    `?cal=http://127.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"private": {
			source:     calExample,
			options:    `?cal=http://192.168.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"vpn": {
			source:     calExample,
			options:    `?cal=http://10.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"localhost": {
			source:     calExample,
			options:    `?cal=http://localhost:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"no-port-localhost": {
			source:     calExample,
			options:    `?cal=http://localhost&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			wantStatus: 502,
		},
		"webcal": {
			source:        calExample,
			options:       `?cal=webcal://CALURL`,
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calExample,
		},
		"ftp": {
			source:        calExample,
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
		"sortsevents": {
			source:        calShuffled,
			options:       "?cal=http://CALURL",
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calExample,
		},
		"eventwithnostart": {
			source:        calEventWithNoStart,
			options:       "?cal=http://CALURL",
			allowLoopback: true,
			wantStatus:    400,
		},
		"dontmerge": {
			source:        calUnmerged,
			options:       "?cal=http://CALURL",
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calUnmerged,
		},
		"merge": {
			source:        calUnmerged,
			options:       "?cal=http://CALURL&mrg=true",
			allowLoopback: true,
			wantStatus:    200,
			wantCal:       calMerged,
		},
		"htmx_asset": {
			options:    "assets/js/htmx.min.js",
			wantStatus: 200,
			wantBody: func() []byte {
				b, err := assets.Assets.ReadFile("js/htmx.min.js")
				require.NoError(t, err)
				return b
			}(),
		},
		"html_index": {
			headers:         map[string]string{"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8"},
			wantStatus:      200,
			wantHTTPRenders: map[string]any{"index": nil},
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
			url := "http://localhost/" + strings.Replace(test.options, "CALURL", tsURL.Host, 1)
			t.Log(url)
			r := httptest.NewRequest("GET", url, nil)
			for k, v := range test.headers {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			defer func(prev bool) { allowLoopback = prev }(allowLoopback)
			allowLoopback = test.allowLoopback

			templ := &mockTemplate{}
			templ.Test(t)
			defer templ.AssertExpectations(t)
			for template, data := range test.wantHTTPRenders {
				rend := &mockRender{}
				rend.Test(t)
				defer rend.AssertExpectations(t)
				rend.On("Render", mock.Anything).Return(nil).Once()
				templ.On("Instance", template, data).Return(rend).Once()
			}

			gin.SetMode(gin.TestMode)
			router := gin.New()
			New(router)
			router.HTMLRender = templ
			router.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantCal != nil {
				wantCal, err := ics.ParseCalendar(bytes.NewReader(test.wantCal))
				require.NoError(t, err)
				assert.Equal(t, []byte(wantCal.Serialize()), w.Body.Bytes())
				assert.Equal(t, "text/calendar", w.Header().Get("Content-Type"))
				t.Logf("want:\n%s", wantCal.Serialize())
				t.Logf("got:\n%s", w.Body.String())
			}
			if test.wantBody != nil {
				require.Equal(t, test.wantBody, w.Body.Bytes())
			}
		})
	}
}

type mockTemplate struct {
	mock.Mock
}

func (m *mockTemplate) Instance(template string, data any) render.Render {
	return m.Called(template, data).Get(0).(render.Render)
}

type mockRender struct {
	mock.Mock
}

func (m *mockRender) Render(w http.ResponseWriter) error {
	return m.Called(w).Error(0)
}

func (m *mockRender) WriteContentType(w http.ResponseWriter) {
	m.Called(w)
}
