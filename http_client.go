package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

var (
	dialer = &net.Dialer{}
	client = &http.Client{
		Timeout: time.Minute,
		Transport: &http.Transport{
			DialContext: publicUnicastOnlyDialContext,
		},
		CheckRedirect: noRedirect,
	}
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
	if ip == nil {
		ipAddrs, err := net.LookupHost(host)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup host: %w", err)
		}
		for _, ipAddr := range ipAddrs {
			ip = net.ParseIP(ipAddr)
			if ip != nil {
				break
			}
		}
	}
	switch {
	case allowLoopback && ip.IsLoopback():
	case !ip.IsGlobalUnicast() || ip.IsPrivate():
		return nil, fmt.Errorf("denied access to unsafe address: %s", ip.String())
	}

	return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}
