package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	server "github.com/brackendawson/webcal-proxy"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		addr     string
		logFile  string
		logLevel string
	)
	flag.StringVar(&logFile, "logfile", "", "File to log to")
	flag.StringVar(&logLevel, "loglevel", "info", "log level")
	flag.StringVar(&addr, "addr", "0.0.0.0:80", "local address:port to bind to")
	flag.Parse()

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid log level")
		os.Exit(1)
	}
	logrus.SetLevel(level)

	if logFile != "" {
		logFH, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to open log file: ", err)
			os.Exit(1)
		}
		defer logFH.Close()
		logrus.SetOutput(logFH)
	}

	s := server.Server{}

	http.HandleFunc("/", s.HandleWebcal)
	logrus.Error(http.ListenAndServe(addr, nil))
}
