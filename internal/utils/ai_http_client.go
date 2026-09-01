package utils

import (
	"net"
	"net/http"
	"time"
)

func NewAIHttpClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        300,

		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,

		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,

		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}
}

func NewAIHttpClientNoTimeout() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        300,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}