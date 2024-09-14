package server_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
	server "github.com/brackendawson/webcal-proxy"
	"github.com/brackendawson/webcal-proxy/assets"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logrus.SetLevel(logrus.DebugLevel)

	for name, test := range map[string]struct {
		// input request
		inputMethod  string
		inputQuery   string
		inputHeaders map[string]string
		inputBody    []byte
		inputCache   *server.Cache

		// server settings
		serverOpts []server.Opt

		// upstream server double
		upstreamServer http.HandlerFunc

		// assertions
		expectedStatus       int
		expectedCalendar     []byte
		expectedBody         []byte
		expectedTemplateName string
		expectedTemplateObj  any
	}{
		"default": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calExample,
		},
		"input_inc_validated_before_upstream_request": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL&inc=hjklkhkjh",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			expectedStatus: http.StatusBadRequest,
		},
		"userAgent": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{
					Transport: &server.WithUserAgent{RoundTripper: &http.Transport{}, UserAgent: "blah/1"},
				}),
			},
			upstreamServer: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "blah/1", r.Header.Get("User-Agent"))
			},
			expectedStatus: http.StatusBadGateway,
		},
		"utf8": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer:   mockWebcalServer(http.StatusOK, map[string]string{"Content-Type": "text/calendar; charset=utf-8"}, calExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calExample,
		},
		"not_utf8": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer: mockWebcalServer(http.StatusOK, map[string]string{"Content-Type": ""}, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"not_calendar": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer: mockWebcalServer(http.StatusOK, map[string]string{"Content-Type": "text/html"}, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"not_working": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer: mockWebcalServer(http.StatusInternalServerError, nil, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"no-cal": {
			inputMethod:    http.MethodGet,
			inputQuery:     "?not=right",
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus: http.StatusBadRequest,
		},
		"includeRotation": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL&inc=SUMMARY=Rotation",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calOnlyRotation,
		},
		"excludeSecondary": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL&exc=SUMMARY=Secondary",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calWithoutSecondary,
		},
		"includeExclude": {
			inputMethod:      http.MethodGet,
			inputQuery:       `?cal=http://CALURL&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calMay22NotRotation,
		},
		"local": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://127.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"private": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://192.168.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"vpn": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://10.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"localhost": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://localhost:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"no-port-localhost": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://localhost&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus: http.StatusBadGateway,
		},
		"webcal": {
			inputMethod:      http.MethodGet,
			inputQuery:       `?cal=webcal://CALURL`,
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calExample,
		},
		"ftp": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=ftp://CALURL`,
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calExample),
			expectedStatus: http.StatusBadRequest,
		},
		"unresolvable": {
			inputMethod:    http.MethodGet,
			inputQuery:     "?cal=webcal://not.a.domain",
			expectedStatus: http.StatusBadGateway,
		},
		"sortsevents": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calShuffled),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calExample,
		},
		"eventwithnostart": {
			inputMethod:    http.MethodGet,
			inputQuery:     "?cal=http://CALURL",
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer: mockWebcalServer(http.StatusOK, nil, calEventWithNoStart),
			expectedStatus: http.StatusBadRequest,
		},
		"dontmerge": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calUnmerged),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calUnmerged,
		},
		"merge": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL&mrg=true",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, calUnmerged),
			expectedStatus:   http.StatusOK,
			expectedCalendar: calMerged,
		},
		"htmx_asset": {
			inputMethod:    http.MethodGet,
			inputQuery:     "assets/js/htmx.min.js",
			expectedStatus: http.StatusOK,
			expectedBody: func() []byte {
				b, err := assets.Assets.ReadFile("js/htmx.min.js")
				require.NoError(t, err)
				return b
			}(),
		},
		"html_index": {
			inputMethod:          http.MethodGet,
			inputHeaders:         map[string]string{"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8"},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "index",
			expectedTemplateObj:  nil,
		},
		"htmx_calendar": {
			inputMethod: http.MethodPost,
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View:  server.ViewMonth,
				Title: "September 2024",
				Days:  month11Sept2024,
			},
		},
		"htmx_calendar_with_user_tz": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"user-tz": []string{"America/New_York"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 12, 1, 0, 0, 0, time.UTC) }),
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View:  server.ViewMonth,
				Title: "September 2024",
				Days:  month11Sept2024,
			},
		},
		"htmx_calendar_with_events": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View:  server.ViewMonth,
				Title: "September 2024",
				Days:  month11Sept2024WithEvents,
				Cache: &server.Cache{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
			},
		},
		"input_exc_validation_before_upstream_request": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
				"exc": []string{"falafel"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			expectedStatus: http.StatusBadRequest,
		},
		"htmx_calendar_with_events_and_invalid_cache": {
			// if a bad cache was passed, fetch the upstream and set a cache
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal":        []string{"webcal://CALURL"},
				"ical-cache": []string{"I'm no cache"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View:  server.ViewMonth,
				Title: "September 2024",
				Days:  month11Sept2024WithEvents,
				Cache: &server.Cache{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
			},
		},
		"htmx_calendar_with_events_and_cache": {
			// if a cached calendar was passed, don't fetch the URL or return
			// the cache
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
			}.Encode()),
			inputCache: &server.Cache{
				URL: "webcal://CALURL",
				Calendar: func() *ics.Calendar {
					c, err := ics.ParseCalendar(bytes.NewReader(events11Sept2024))
					require.NoError(t, err)
					return c
				}(),
			},
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View:  server.ViewMonth,
				Title: "September 2024",
				Days:  month11Sept2024WithEvents,
			},
		},
		"htmx_calendar_with_events_and_old_cache": {
			// if a cached calendar was passed that doesn't match the URL, do
			// fetch the URL and the new cache
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
			}.Encode()),
			inputCache: &server.Cache{
				URL: "webcal://boring.co/events",
				Calendar: func() *ics.Calendar {
					c, err := ics.ParseCalendar(bytes.NewReader(events11Sept2024))
					require.NoError(t, err)
					return c
				}(),
			},
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View:  server.ViewMonth,
				Title: "September 2024",
				Days:  month11Sept2024WithEvents,
				Cache: &server.Cache{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
			},
		},
		"htmx_calendar_no_calendar_requested_and_old_cache": {
			// if a cached calendar was passed but no calendar was requested,
			// don't fetch ant URL, and don't set a cache. THe existing cache
			// may remain.
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{""},
			}.Encode()),
			inputCache: &server.Cache{
				URL: "webcal://boring.co/events",
				Calendar: func() *ics.Calendar {
					c, err := ics.ParseCalendar(bytes.NewReader(events11Sept2024))
					require.NoError(t, err)
					return c
				}(),
			},
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View:  server.ViewMonth,
				Title: "September 2024",
				Days:  month11Sept2024,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			upstreamServer := httptest.NewServer(test.upstreamServer)
			defer upstreamServer.Close()

			upstreamURL, err := url.Parse(upstreamServer.URL)
			require.NoError(t, err)

			inputURL := "http://localhost/" + strings.Replace(test.inputQuery, "CALURL", upstreamURL.Host, -1)
			t.Log(inputURL)
			if calendar, ok := test.expectedTemplateObj.(server.Calendar); ok && calendar.Cache != nil {
				calendar.Cache.URL = strings.Replace(calendar.Cache.URL, "CALURL", upstreamURL.Host, -1)
			}
			if test.inputCache != nil {
				test.inputCache.URL = strings.Replace(test.inputCache.URL, "CALURL", upstreamURL.Host, -1)
				q, err := url.ParseQuery(string(test.inputBody))
				require.NoError(t, err, "test.inputCache must only be used when test.inputBody is application/x-www-form-urlencoded")
				cache, err := test.inputCache.Encode()
				require.NoError(t, err)
				q.Set("ical-cache", cache)
				test.inputBody = []byte(q.Encode())
			}
			r := httptest.NewRequest(test.inputMethod, inputURL, bytes.NewReader(bytes.Replace(test.inputBody, []byte("CALURL"), []byte(upstreamURL.Host), -1)))

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

func mockWebcalServer(code int, headers map[string]string, calendar []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(code)
		_, _ = w.Write(calendar)
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
