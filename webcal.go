package server

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"slices"

	ics "github.com/arran4/golang-ical"
	"github.com/brackendawson/webcal-proxy/cache"
)

func parseURLScheme(ctx context.Context, addr string) (string, error) {
	addrURL, err := url.Parse(addr)
	if err != nil {
		log(ctx).Warnf("invalid calendar url: %s", err)
		return "", newErrorWithMessage(
			http.StatusBadRequest,
			"Bad url. Include a protocol, host, and path, eg: webcal://example.com/events",
		)
	}

	if addrURL.Scheme == "webcal" {
		addrURL.Scheme = "http"
	}

	if !slices.Contains([]string{"http", "https"}, addrURL.Scheme) {
		return "", newErrorWithMessage(
			http.StatusBadRequest,
			"Unsupported protocol scheme, url should be webcal, https, or http.",
		)
	}

	return addrURL.String(), nil
}

func (s *Server) getUpstreamCalendar(ctx context.Context, url string) (*ics.Calendar, error) {
	upstreamURL, err := parseURLScheme(ctx, url)
	if err != nil {
		return nil, err
	}

	upstream, err := s.fetch(upstreamURL)
	if err != nil {
		log(ctx).Warnf("Failed to fetch calendar %q: %s", upstreamURL, err)
		return nil, newErrorWithMessage(
			http.StatusBadGateway,
			"Failed to fetch calendar",
		)
	}

	return upstream, nil
}

func decodeCache(ctx context.Context, upstreamURL string, rawCache string) (*ics.Calendar, bool) {
	if rawCache == "" {
		return nil, false
	}
	cache, err := cache.ParseWebcal(rawCache)
	if err != nil {
		log(ctx).Warnf("Failed to parse cache: %s. Continuing without.")
		return nil, false
	}

	if cache.URL != upstreamURL {
		log(ctx).Debugf("Cache was for old URL %q, ignoring.", cache.URL)
		return nil, false
	}

	log(ctx).Debug("Using cached calendar")
	return cache.Calendar, true
}

func (s *Server) getUpstreamWithCache(ctx context.Context, upstreamURL string, rawCache string) (_ *ics.Calendar, usedCache bool, _ error) {
	upstream, ok := decodeCache(ctx, upstreamURL, rawCache)
	if !ok {
		upstream, err := s.getUpstreamCalendar(ctx, upstreamURL)
		if err != nil {
			return nil, false, err
		}

		return upstream, false, nil
	}

	return upstream, true, nil
}

// fetch fetches the given url from the given IP address
func (s *Server) fetch(url string) (*ics.Calendar, error) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	upstream, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer upstream.Body.Close()
	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		return nil, fmt.Errorf("bad status: %s", upstream.Status)
	}
	contentType := upstream.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return nil, fmt.Errorf("error parsing content type: %w", err)
		}
		if mediaType != "text/calendar" {
			return nil, errors.New("not a calendar")
		}
	}

	return ics.ParseCalendar(upstream.Body)
}
