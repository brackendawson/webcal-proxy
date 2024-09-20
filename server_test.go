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
	"github.com/brackendawson/webcal-proxy/cache"
	"github.com/brackendawson/webcal-proxy/fixtures"
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
		inputCache   *cache.Webcal

		// server settings
		serverOpts []server.Opt

		// upstream server double
		upstreamServer http.HandlerFunc

		// assertions
		expectedStatus       int
		expectedCalendar     []byte
		expectedBody         *[]byte
		expectedTemplateName string
		expectedTemplateObj  any
		expectedHeaders      map[string]string
	}{
		"default": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalExample,
		},
		"input_inc_validated_before_upstream_request": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL&inc=hjklkhkjh",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   ptrTo([]byte(`Bad inc argument: invalid match parameter "hjklkhkjh" at index 0, should be <FIELD>=<regexp>`)),
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
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.EmptyCalendar,
		},
		"utf8": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer:   mockWebcalServer(http.StatusOK, map[string]string{"Content-Type": "text/calendar; charset=utf-8"}, fixtures.CalExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalExample,
		},
		"no_content_type": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer:   mockWebcalServer(http.StatusOK, map[string]string{"Content-Type": ""}, fixtures.CalExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalExample,
		},
		"not_calendar": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer: mockWebcalServer(http.StatusOK, map[string]string{"Content-Type": "text/html"}, fixtures.CalExample),
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"not_working": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=http://CALURL",
			serverOpts: []server.Opt{
				server.WithUnsafeClient(&http.Client{}),
				server.MaxConns(1),
			},
			upstreamServer: mockWebcalServer(http.StatusInternalServerError, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"no-cal": {
			inputMethod:    http.MethodGet,
			inputQuery:     "?not=right",
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer: mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   ptrTo([]byte(`Missing "cal" parameter, must be a webcal URL.`)),
		},
		"includeRotation": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL&inc=SUMMARY=Rotation",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalOnlyRotation,
		},
		"excludeSecondary": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL&exc=SUMMARY=Secondary",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalWithoutSecondary,
		},
		"includeExclude": {
			inputMethod:      http.MethodGet,
			inputQuery:       `?cal=http://CALURL&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalMay22NotRotation,
		},
		"local": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://127.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"private": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://192.168.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"vpn": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://10.0.0.1:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"localhost": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://localhost:80&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"no-port-localhost": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=http://localhost&inc=DTSTART=202205\d\dT&exc=SUMMARY=Rotation`,
			upstreamServer: mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"webcal": {
			inputMethod:      http.MethodGet,
			inputQuery:       `?cal=webcal://CALURL`,
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalExample,
		},
		"ftp": {
			inputMethod:    http.MethodGet,
			inputQuery:     `?cal=ftp://CALURL`,
			serverOpts:     []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer: mockWebcalServer(http.StatusOK, nil, fixtures.CalExample),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   ptrTo([]byte("Unsupported protocol scheme, url should be webcal, https, or http.")),
		},
		"unresolvable": {
			inputMethod:    http.MethodGet,
			inputQuery:     "?cal=webcal://not.a.domain",
			expectedStatus: http.StatusBadGateway,
			expectedBody:   ptrTo([]byte("Failed to fetch calendar")),
		},
		"sorts_events": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalShuffled),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalExample,
		},
		"event_with_no_start": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalEventWithNoStart),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalEventWithNoStartSorted,
		},
		"do_not_merge": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalUnMerged),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalUnMerged,
		},
		"merge": {
			inputMethod:      http.MethodGet,
			inputQuery:       "?cal=http://CALURL&mrg=true",
			serverOpts:       []server.Opt{server.WithUnsafeClient(&http.Client{})},
			upstreamServer:   mockWebcalServer(http.StatusOK, nil, fixtures.CalUnMerged),
			expectedStatus:   http.StatusOK,
			expectedCalendar: fixtures.CalMerged,
		},
		"htmx_asset": {
			inputMethod:    http.MethodGet,
			inputQuery:     "assets/js/htmx.min.js",
			expectedStatus: http.StatusOK,
			expectedBody: func() *[]byte {
				b, err := assets.Assets.ReadFile("js/htmx.min.js")
				require.NoError(t, err)
				return &b
			}(),
		},
		"html_index": {
			inputMethod:          http.MethodGet,
			inputHeaders:         map[string]string{"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8"},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "index",
			expectedTemplateObj: server.Index{
				View: server.View{
					ArgHost: "example.com",
				},
			},
		},
		"html_index_behind_reverse_proxy": {
			inputMethod: http.MethodGet,
			inputHeaders: map[string]string{
				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/png,image/svg+xml,*/*;q=0.8",
				"X-Forwarded-URI": "/webcal-proxy",
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "index",
			expectedTemplateObj: server.Index{
				View: server.View{
					ArgHost:      "example.com",
					ArgProxyPath: "/webcal-proxy",
				},
			},
		},
		"htmx_calendar": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024,
			},
		},
		"htmx_calendar_with_user_tz": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
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
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024,
			},
		},
		"htmx_calendar_with_events": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, fixtures.Events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024WithEvents,
				Cache: &cache.Webcal{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
				URL: "webcal://example.com?cal=webcal%3A%2F%2FCALURL",
			},
		},
		"htmx_calendar_with_events_behind_reverse_proxy": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":       "example.com",
				"Content-Type":    "application/x-www-form-urlencoded",
				"X-Forwarded-URI": "/webcal-proxy",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, fixtures.Events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost:      "example.com",
					ArgProxyPath: "/webcal-proxy",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024WithEvents,
				Cache: &cache.Webcal{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
				URL: "webcal://example.com/webcal-proxy?cal=webcal%3A%2F%2FCALURL",
			},
		},
		"htmx_calendar_with_events_and_local_time": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal":     []string{"webcal://CALURL"},
				"user-tz": []string{"Australia/Sydney"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, fixtures.Events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024WithEventsSydney,
				Cache: &cache.Webcal{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
				URL: "webcal://example.com?cal=webcal%3A%2F%2FCALURL",
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
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024,
				Error:        "Bad exc argument: invalid match parameter \"falafel\" at index 0, should be <FIELD>=<regexp>",
			},
		},
		"bad_url": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://\\\\CALURL"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024,
				Error:        "Bad url. Include a protocol, host, and path, eg: webcal://example.com/events",
			},
		},
		"bad_url_percent": {
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"%"},
			}.Encode()),
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024,
				Error:        "Bad url. Include a protocol, host, and path, eg: webcal://example.com/events",
			},
		},
		"htmx_calendar_with_events_and_invalid_cache": {
			// if a bad cache was passed, fetch the upstream and set a cache
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
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
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, fixtures.Events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024WithEvents,
				Cache: &cache.Webcal{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
				URL: "webcal://example.com?cal=webcal%3A%2F%2FCALURL",
			},
		},
		"htmx_calendar_with_events_and_cache": {
			// if a cached calendar was passed, don't fetch the URL or return
			// the cache
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
			}.Encode()),
			inputCache: &cache.Webcal{
				URL: "webcal://CALURL",
				Calendar: func() *ics.Calendar {
					c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
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
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024WithEvents,
				URL:          "webcal://example.com?cal=webcal%3A%2F%2FCALURL",
			},
		},
		"htmx_calendar_with_events_and_old_cache": {
			// if a cached calendar was passed that doesn't match the URL, do
			// fetch the URL and the new cache
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{"webcal://CALURL"},
			}.Encode()),
			inputCache: &cache.Webcal{
				URL: "webcal://boring.co/events",
				Calendar: func() *ics.Calendar {
					c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
					require.NoError(t, err)
					return c
				}(),
			},
			serverOpts: []server.Opt{
				server.WithClock(func() time.Time { return time.Date(2024, 9, 11, 23, 0, 0, 0, time.UTC) }),
				server.WithUnsafeClient(&http.Client{}),
			},
			upstreamServer:       mockWebcalServer(http.StatusOK, nil, fixtures.Events11Sept2024),
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "calendar",
			expectedTemplateObj: server.Calendar{
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024WithEvents,
				Cache: &cache.Webcal{
					URL: "webcal://CALURL",
					Calendar: func() *ics.Calendar {
						c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
						require.NoError(t, err)
						return c
					}(),
				},
				URL: "webcal://example.com?cal=webcal%3A%2F%2FCALURL",
			},
		},
		"htmx_calendar_no_calendar_requested_and_old_cache": {
			// if a cached calendar was passed but no calendar was requested,
			// don't fetch ant URL, and don't set a cache. THe existing cache
			// may remain.
			inputMethod: http.MethodPost,
			inputHeaders: map[string]string{
				"X-HX-Host":    "example.com",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			inputBody: []byte(url.Values{
				"cal": []string{""},
			}.Encode()),
			inputCache: &cache.Webcal{
				URL: "webcal://boring.co/events",
				Calendar: func() *ics.Calendar {
					c, err := ics.ParseCalendar(bytes.NewReader(fixtures.Events11Sept2024))
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
				View: server.View{
					ArgHost: "example.com",
				},
				CalendarView: server.ViewMonth,
				Title:        "September 2024",
				Days:         month11Sept2024,
			},
		},
		"add_matcher_group": {
			inputMethod: http.MethodGet,
			inputQuery:  "matcher",
			inputHeaders: map[string]string{
				"X-HX-Host":       "example.com",
				"X-Forwarded-URI": "/webcal-proxy",
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "template-matcher-group",
			expectedTemplateObj: server.View{
				ArgHost:      "example.com",
				ArgProxyPath: "/webcal-proxy",
			},
		},
		"remove_matcher_group": {
			inputMethod: http.MethodDelete,
			inputQuery:  "matcher",
			inputHeaders: map[string]string{
				"X-HX-Host":       "example.com",
				"X-Forwarded-URI": "/webcal-proxy",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   ptrTo([]byte(nil)),
			expectedHeaders: map[string]string{
				"HX-Trigger-After-Settle": `{"input":{"target":"#trigger-submit"}}`,
			},
		},
		"pre_fills_the_form_from_url": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=webcal%3A%2F%2Fyolo.com%2Fevents.ics&inc=SUMMARY%3Dinteresting&inc=SUMMARY%3Dmiddling&exc=DESCRIPTION%3Dboring&mrg=true",
			inputHeaders: map[string]string{
				"Accept": "text/html",
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "index",
			expectedTemplateObj: server.Index{
				View: server.View{
					ArgHost: "example.com",
				},
				Options: server.Options{
					URL: "webcal://yolo.com/events.ics",
					Includes: []server.Matcher{
						{Property: "SUMMARY", Regex: "interesting"},
						{Property: "SUMMARY", Regex: "middling"},
					},
					Excludes: []server.Matcher{
						{Property: "DESCRIPTION", Regex: "boring"},
					},
					Merge: true,
				},
			},
		},

		"fails_to_pre_fill_the_form": {
			inputMethod: http.MethodGet,
			inputQuery:  "?cal=webcal%3A%2F%2Fyolo.com%2Fevents.ics&mrg=yes",
			inputHeaders: map[string]string{
				"Accept": "text/html",
			},
			expectedStatus:       http.StatusOK,
			expectedTemplateName: "index",
			expectedTemplateObj: server.Index{
				View: server.View{
					ArgHost: "example.com",
				},
				Error: `Bad argument "yes" for "mrg", should be boolean. Enter your webcal URL.`,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			upstreamServer := httptest.NewServer(test.upstreamServer)
			defer upstreamServer.Close()

			upstreamURL, err := url.Parse(upstreamServer.URL)
			require.NoError(t, err)

			inputURL := "/" + strings.Replace(test.inputQuery, "CALURL", upstreamURL.Host, -1)
			t.Log(inputURL)
			if calendar, ok := test.expectedTemplateObj.(server.Calendar); ok {
				if calendar.Cache != nil {
					calendar.Cache.URL = strings.Replace(calendar.Cache.URL, "CALURL", upstreamURL.Host, -1)
				}
				calendar.URL = strings.Replace(calendar.URL, "CALURL", url.QueryEscape(upstreamURL.Host), -1)
				test.expectedTemplateObj = calendar
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
			for k, v := range test.expectedHeaders {
				require.Equal(t, v, w.Header().Get(k))
			}
			if test.expectedCalendar != nil {
				expectedCalendar, err := ics.ParseCalendar(bytes.NewReader(test.expectedCalendar))
				require.NoError(t, err)
				assert.Equal(t, "text/calendar", w.Header().Get("Content-Type"))
				assert.Equal(t, []byte(expectedCalendar.Serialize()), w.Body.Bytes())
				t.Logf("expected:\n%s", expectedCalendar.Serialize())
				t.Logf("actual:\n%s", w.Body.String())
			}
			if test.expectedBody != nil {
				require.Equal(t, *test.expectedBody, w.Body.Bytes())
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

func ptrTo[T any](t T) *T {
	return &t
}
