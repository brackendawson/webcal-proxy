package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	requestTimeoutSecs = 60
)

var (
	dialer = &net.Dialer{}
	client = &http.Client{
		Timeout: requestTimeoutSecs * time.Second,
		Transport: &http.Transport{
			DialContext: publicUnicastOnlyDialContext,
		},
		CheckRedirect: noRedirect,
	}
	resolver = &net.Resolver{}

	allowLoopback = false
)

func noRedirect(*http.Request, []*http.Request) error {
	return errors.New("redirect is not allowed")
}

func publicUnicastOnlyDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(host)

	var ipAddrs []string
	if ip != nil {
		ipAddrs = append(ipAddrs, ip.String())
	} else {
		ipAddrs, err = resolver.LookupHost(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup host: %w", err)
		}
	}

	var dialErrs []error
	connectTimeout := requestTimeoutSecs * time.Second / time.Duration(len(ipAddrs)+1)
	for _, ipAddr := range ipAddrs {
		ip = net.ParseIP(ipAddr)
		switch {
		case allowLoopback && ip.IsLoopback():
		case !ip.IsGlobalUnicast() || ip.IsPrivate():
			dialErrs = append(dialErrs, fmt.Errorf("forbidden address: %s", ipAddr))
			continue
		}

		connCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		conn, err := dialer.DialContext(connCtx, network, net.JoinHostPort(ipAddr, port))
		cancel()
		if nil == err {
			return conn, nil
		}
		dialErrs = append(dialErrs, fmt.Errorf("failed to dial '%s:%s': %w", ipAddr, port, err))
	}

	return nil, fmt.Errorf("failed to dial: %q", dialErrs)
}
