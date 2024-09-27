package server

import (
	"net/http"
	"time"
)

// WithUnsafeClient allows the HTTP client to be set. No client other than the
// default is safe to use.
func WithUnsafeClient(c *http.Client) Opt {
	return func(s *Server) {
		s.client = c
	}
}

func WithClock(f func() time.Time) Opt {
	return func(s *Server) {
		s.now = f
	}
}
