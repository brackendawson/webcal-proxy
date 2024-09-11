package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-uuid"
	"github.com/sirupsen/logrus"
)

func logging(c *gin.Context) {
	start := time.Now()

	log := logrus.WithFields(logrus.Fields{
		"method": c.Request.Method,
		"url":    c.Request.URL.String(),
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

	c.Set("log", log)

	c.Next()

	for _, err := range c.Errors {
		log.Error(err.Error())
		c.String(http.StatusInternalServerError, err.Error())
	}

	log.WithFields(logrus.Fields{
		"code":     c.Writer.Status(),
		"bytes":    c.Writer.Size(),
		"duration": time.Since(start).Seconds(),
	}).Info("Request complete")
}

func log(c *gin.Context) logrus.FieldLogger {
	v, ok := c.Get("log")
	if !ok {
		return logrus.StandardLogger()
	}

	log, ok := v.(logrus.FieldLogger)
	if !ok {
		return logrus.StandardLogger()
	}

	return log
}
