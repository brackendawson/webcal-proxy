package server

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-uuid"
	"github.com/sirupsen/logrus"
)

type contextKey int

const (
	ctxKeyLogger contextKey = iota
)

type oddometer struct {
	io.ReadCloser
	bytes int
}

func (o *oddometer) Read(p []byte) (int, error) {
	n, err := o.ReadCloser.Read(p)
	o.bytes += n
	return n, err
}

func logging(c *gin.Context) {
	start := time.Now()

	log := logrus.WithFields(logrus.Fields{
		"req_method": c.Request.Method,
		"req_url":    c.Request.URL.String(),
		"req_bytes":  c.GetHeader("Content-Length"),
	})

	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		var err error
		requestID, err = uuid.GenerateUUID()
		if err != nil {
			log.Errorf("Failed to generate UUID for request: %s", err)
		}
	}
	c.Header("X-Request-ID", requestID)
	log = log.WithField("id", requestID)

	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKeyLogger, log))

	requestRead := oddometer{c.Request.Body, 0}
	c.Request.Body = &requestRead

	c.Next()

	for _, err := range c.Errors {
		log.Error(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
	}

	log.WithFields(logrus.Fields{
		"req_read":  requestRead.bytes,
		"res_code":  c.Writer.Status(),
		"res_bytes": c.Writer.Size(),
		"duration":  time.Since(start).Seconds(),
	}).Info("Request complete")
}

func log(ctx context.Context) logrus.FieldLogger {
	log, ok := ctx.Value(ctxKeyLogger).(logrus.FieldLogger)
	if !ok {
		return logrus.StandardLogger()
	}

	return log
}
