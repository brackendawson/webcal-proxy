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
		if matcher.regx.Match([]byte(event.GetProperty(matcher.property).Value)) {
			return true
		}
	}
	return false
}

type matcher struct {
	property ics.ComponentProperty
	regx     *regexp.Regexp
}

func parseMatchers(m []string) (matchGroup, error) {
	matches := make(matchGroup, 0, len(m))
	for _, matchOpt := range m {
		parts := strings.Split(matchOpt, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid match paramater: %s, should be <FIELD>=<regexp>", matchOpt)
		}
		regx, err := regexp.Compile(parts[1])
		if err != nil {
			return nil, fmt.Errorf("bad regexp in match paramater '%s': %s", matchOpt, err)
		}
		matches = append(matches, matcher{
			property: ics.ComponentProperty(parts[0]),
			regx:     regx,
		})
	}
	return matches, nil
}
