package server

import (
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func logging(c *gin.Context) {
	start := time.Now()
	log := logrus.WithFields(logrus.Fields{
		"id":     requestid.Get(c),
		"method": c.Request.Method,
		"url":    c.Request.URL.String(),
	})

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
