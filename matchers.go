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
	for _, matchOpt := range m {
		parts := strings.Split(matchOpt, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid match parameter: %s, should be <FIELD>=<regexp>", matchOpt)
		}
		expression, err := regexp.Compile(parts[1])
		if err != nil {
			return nil, fmt.Errorf("bad regexp in match parameter '%s': %s", matchOpt, err)
		}
		matches = append(matches, matcher{
			property:   ics.ComponentProperty(parts[0]),
			expression: expression,
		})
	}
	return matches, nil
}
