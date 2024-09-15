package server

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	ics "github.com/arran4/golang-ical"
	"github.com/gin-gonic/gin"
)

type calenderOptions struct {
	url                string
	includes, excludes matchGroup
	merge              bool
}

func getCalendarOptions(ctx context.Context, getArray func(string) []string) (calenderOptions, error) {
	var (
		opts calenderOptions
		err  error
	)

	opts.merge, err = getBool(ctx, getArray, "mrg")
	if err != nil {
		return calenderOptions{}, err
	}

	opts.includes, err = parseMatchers(getArray("inc"))
	if err != nil {
		return calenderOptions{}, newErrorWithMessage(
			http.StatusBadRequest,
			"Bad inc argument: %s", err.Error(),
		)
	}
	if len(opts.includes) == 0 {
		opts.includes = matchGroup{
			{
				property:   ics.ComponentPropertySummary,
				expression: regexp.MustCompile(".*"),
			},
		}
	}

	opts.excludes, err = parseMatchers(getArray("exc"))
	if err != nil {
		return calenderOptions{}, newErrorWithMessage(
			http.StatusBadRequest,
			"Bad exc argument: %s", err.Error(),
		)
	}

	opts.url = getString(getArray, "cal")

	return opts, nil
}

func getBool(ctx context.Context, getArray func(string) []string, key string) (bool, error) {
	bs := getArray(key)
	if len(bs) < 1 {
		return false, nil
	}

	b, err := strconv.ParseBool(bs[0])
	if err != nil {
		log(ctx).Warnf("error getting %q parameter: %w", key, err)
		return false, newErrorWithMessage(
			http.StatusBadRequest,
			"Bad argument %q for %q, should be boolean.", bs[0], key,
		)
	}

	return b, nil
}

func getString(getArray func(string) []string, key string) string {
	ss := getArray(key)
	if len(ss) < 1 {
		return ""
	}
	return ss[0]
}

func clientURL(c *gin.Context) *url.URL {
	u := c.Request.URL
	u.Scheme = "webcal"
	u.Host = c.GetHeader("X-HX-Host") // Host header is banned in XHR
	proxyPath := c.GetHeader("X-Forwarded-URI")
	u.Path = proxyPath

	q := url.Values{
		"cal": c.PostFormArray("cal"),
		"inc": c.PostFormArray("inc"),
		"exc": c.PostFormArray("exc"),
		"mrg": c.PostFormArray("mrg"),
	}
	u.RawQuery = q.Encode()

	return u
}
