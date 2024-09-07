package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	server "github.com/brackendawson/webcal-proxy"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		addr     string
		logFile  string
		logLevel logrus.Level
	)
	flag.StringVar(&logFile, "logfile", "", "File to log to")
	flag.TextVar(&logLevel, "log-level", logrus.InfoLevel, "log level")
	flag.StringVar(&addr, "addr", ":80", "local address:port to bind to")
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
	server.New(r)

	logrus.Info("Begin listener...")
	logrus.Fatal(http.ListenAndServe(addr, r))
}
