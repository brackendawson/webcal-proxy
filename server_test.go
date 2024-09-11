package server_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	ics "github.com/arran4/golang-ical"
	server "github.com/brackendawson/webcal-proxy"
	"github.com/brackendawson/webcal-proxy/assets"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for name, test := range map[string]struct {
		// input request
		inputQuery   string
		inputHeaders map[string]string

		// server settings
		serverOpts []server.Opt

		// upstream server double
		upstreamStatus  int
		upstreamHeaders map[string]string
		upstreamBody    []byte

		// assertions
		expectedStatus       int
		expectedCalendar     []byte
		expectedBody         []byte
		expectedTemplateName string
		expectedTemplateObj  any
	}{
		"default": {
			inputQuery: "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calExample,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calExample,
		},
		"no-cal": {
			inputQuery:     "?not=right",
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus: http.StatusOK,
			upstreamBody:   calExample,
			expectedStatus: http.StatusBadRequest,
		},
		"includeRotation": {
			inputQuery:       "?cal=http://CALURL&inc=SUMMARY=Rotation",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calExample,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calOnlyRotation,
		},
		"excludeSecondary": {
			inputQuery:       "?cal=http://CALURL&exc=SUMMARY=Secondary",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calExample,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calWithoutSecondary,
		},
		"includeExclude": {
			inputQuery:       `?cal=http://CALURL&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calExample,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calMay22NotRotation,
		},
		"local": {
			inputQuery:     `?cal=http://127.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamStatus: http.StatusOK,
			upstreamBody:   calExample,
			expectedStatus: 502,
		},
		"private": {
			inputQuery:     `?cal=http://192.168.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamStatus: http.StatusOK,
			upstreamBody:   calExample,
			expectedStatus: 502,
		},
		"vpn": {
			inputQuery:     `?cal=http://10.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamStatus: http.StatusOK,
			upstreamBody:   calExample,
			expectedStatus: 502,
		},
		"localhost": {
			inputQuery:     `?cal=http://localhost:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamStatus: http.StatusOK,
			upstreamBody:   calExample,
			expectedStatus: 502,
		},
		"no-port-localhost": {
			inputQuery:     `?cal=http://localhost&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamStatus: http.StatusOK,
			upstreamBody:   calExample,
			expectedStatus: 502,
		},
		"webcal": {
			inputQuery:       `?cal=webcal://CALURL`,
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calExample,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calExample,
		},
		"ftp": {
			inputQuery:     `?cal=ftp://CALURL`,
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus: http.StatusOK,
			upstreamBody:   calExample,
			expectedStatus: http.StatusBadRequest,
		},
		"redirect": {
			inputQuery: `?cal=webcal://CALURL`,
			// serverOpts:      []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:  http.StatusMovedPermanently,
			upstreamHeaders: map[string]string{"Location": "http://192.168.0.1"},
			expectedStatus:  502,
		},
		"unresolvable": {
			inputQuery:     "?cal=webcal://not.a.domain",
			expectedStatus: 502,
		},
		"sortsevents": {
			inputQuery:       "?cal=http://CALURL",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calShuffled,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calExample,
		},
		"eventwithnostart": {
			inputQuery:     "?cal=http://CALURL",
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus: http.StatusOK,
			upstreamBody:   calEventWithNoStart,
			expectedStatus: http.StatusBadRequest,
		},
		"dontmerge": {
			inputQuery:       "?cal=http://CALURL",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calUnmerged,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calUnmerged,
		},
		"merge": {
			inputQuery:       "?cal=http://CALURL&mrg=true",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamStatus:   http.StatusOK,
			upstreamBody:     calUnmerged,
			expectedStatus:   http.StatusOK,
			expectedCalendar: calMerged,
		},
		"htmx_asset": {
			inputQuery:     "assets/js/htmx.min.js",
			expectedStatus: http.StatusOK,
			expectedBody: func() []byte {
				b, err := assets.Assets.ReadFile("js/htmx.min.js")
				require.NoError(t, err)
				return b
			}(),
		},
		"html_index": {
			inputHeaders:         map[string]string{"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8"},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "index",
			expectedTemplateObj:  nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range test.upstreamHeaders {
					w.Header().Set(k, v)
				}
				w.Header().Set("Content-Type", "text/calendar")
				w.WriteHeader(test.upstreamStatus)
				_, _ = w.Write(test.upstreamBody)
			}))
			defer upstreamServer.Close()

			upstreamURL, err := url.Parse(upstreamServer.URL)
			require.NoError(t, err)
			inputURL := "http://localhost/" + strings.Replace(test.inputQuery, "CALURL", upstreamURL.Host, 1)
			t.Log(inputURL)
			r := httptest.NewRequest("GET", inputURL, nil)
			for k, v := range test.inputHeaders {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			tpl := &mockTemplate{}
			tpl.Test(t)
			defer tpl.AssertExpectations(t)
			if test.expectedTemplateName != "" {
				rend := &mockRender{}
				rend.Test(t)
				defer rend.AssertExpectations(t)
				rend.On("Render", mock.Anything).Return(nil).Once()
				tpl.On("Instance", test.expectedTemplateName, test.expectedTemplateObj).Return(rend).Once()
			}

			router := gin.New()
			server.New(router, test.serverOpts...)
			router.HTMLRender = tpl
			router.ServeHTTP(w, r)

			assert.Equal(t, test.expectedStatus, w.Code)
			if test.expectedCalendar != nil {
				expectedCalendar, err := ics.ParseCalendar(bytes.NewReader(test.expectedCalendar))
				require.NoError(t, err)
				assert.Equal(t, "text/calendar", w.Header().Get("Content-Type"))
				assert.Equal(t, []byte(expectedCalendar.Serialize()), w.Body.Bytes())
				t.Logf("expected:\n%s", expectedCalendar.Serialize())
				t.Logf("actual:\n%s", w.Body.String())
			}
			if test.expectedBody != nil {
				require.Equal(t, test.expectedBody, w.Body.Bytes())
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

func must[T, U any](t *testing.T, f func(T) (U, error), v T) U {
	u, err := f(v)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
