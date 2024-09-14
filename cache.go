package server

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"

	ics "github.com/arran4/golang-ical"
)

type Cache struct {
	URL      string
	Calendar *ics.Calendar
}

func ParseCache(c string) (Cache, error) {
	rawGzip, err := base64.StdEncoding.DecodeString(c)
	if err != nil {
		return Cache{}, fmt.Errorf("error decoding cache: %w", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(rawGzip))
	if err != nil {
		return Cache{}, fmt.Errorf("error decoding cache headers: %w", err)
	}
	calendarBytes, err := io.ReadAll(r)
	if err != nil {
		return Cache{}, fmt.Errorf("error decoding cache body: %w", err)
	}
	cache := Cache{
		URL: r.Name,
	}
	if cache.Calendar, err = ics.ParseCalendar(bytes.NewReader(calendarBytes)); err != nil {
		return Cache{}, fmt.Errorf("error parsing cached calendar: %w", err)
	}
	return cache, nil
}

func (c Cache) Encode() (string, error) {
	b := bytes.NewBuffer([]byte{})
	w, err := gzip.NewWriterLevel(b, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("error creating cache: %w", err)
	}
	w.Name = c.URL
	if _, err := w.Write([]byte(c.Calendar.Serialize())); err != nil {
		return "", fmt.Errorf("error encoding cache: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("error finalising cache: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}
