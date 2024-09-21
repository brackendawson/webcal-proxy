package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	server "github.com/brackendawson/webcal-proxy"
	"github.com/gin-contrib/secure"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		addr         string
		logFile      string
		logLevel     logrus.Level
		secureConfig secure.Config = secure.DefaultConfig()
		maxConns     int
	)
	flag.StringVar(&logFile, "log-file", "", "File to log to")
	flag.TextVar(&logLevel, "log-level", logrus.InfoLevel, "log level")
	flag.BoolVar(&secureConfig.IsDevelopment, "dev", false, "disables security policies that prevent http://localhost from working")
	flag.StringVar(&addr, "addr", ":8080", "local address:port to bind to")
	flag.IntVar(&maxConns, "max-conns", 8, "maximum total upstream connections")
	flag.Parse()

	logrus.SetLevel(logLevel)

	if logFile != "" {
		logFH, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to open log file: ", err)
			os.Exit(1)
		}
		defer logFH.Close()
		logrus.SetOutput(logFH)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.RedirectTrailingSlash = false // be permissie, gin is not aware of Proxy Path
	r.RedirectFixedPath = true
	secureConfig.SSLRedirect = false                                                                                  // TLS should be handled by reverse proxy
	secureConfig.ContentSecurityPolicy = "default-src 'self'; script-src 'self' 'unsafe-eval'; img-src 'self' data:;" // HTMX event filters need eval
	r.Use(secure.New(secureConfig))
	server.New(r, server.MaxConns(maxConns))

	if secureConfig.IsDevelopment {
		logrus.Warn("In development mode, some security policies disabled to allow http://localhost/ to work.")
	}
	logrus.Info("Begin listener...")
	logrus.Fatal(http.ListenAndServe(addr, r))
}
