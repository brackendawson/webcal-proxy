package server

import (
	"fmt"
	"regexp"
	"strings"

	ics "github.com/arran4/golang-ical"
)

type matchGroup []matcher

func (m matchGroup) matches(event *ics.VEvent) bool {
	for _, matcher := range m {
		if matcher.expression.Match([]byte(event.GetProperty(matcher.property).Value)) {
			return true
		}
	}
	return false
}

type matcher struct {
	property   ics.ComponentProperty
	expression *regexp.Regexp
}

func parseMatchers(m []string) (matchGroup, error) {
	matches := make(matchGroup, 0, len(m))
	for i, matchOpt := range m {
		parts := strings.Split(matchOpt, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid match parameter %q at index %d, should be <FIELD>=<regexp>", matchOpt, i)
		}
		expression, err := regexp.Compile(parts[1])
		if err != nil {
			return nil, fmt.Errorf("bad regexp in match parameter %s at index %d: %w", matchOpt, i, err)
		}
		matches = append(matches, matcher{
			property:   ics.ComponentProperty(parts[0]),
			expression: expression,
		})
	}
	return matches, nil
}
