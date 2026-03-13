package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewSingleHost creates a reverse proxy that forwards requests to upstream.
// Deprecated: Use NewSingleHostE for explicit error handling.
func NewSingleHost(upstream string, rewrite func(*http.Request)) http.Handler {
	h, err := NewSingleHostE(upstream, rewrite)
	if err != nil {
		panic(fmt.Sprintf("proxy: %v", err))
	}
	return h
}

// NewSingleHostE creates a reverse proxy like NewSingleHost but returns an
// error instead of panicking when the upstream URL is invalid.
func NewSingleHostE(upstream string, rewrite func(*http.Request)) (http.Handler, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parse upstream URL %q: %w", upstream, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("upstream URL %q must have scheme and host", upstream)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		rewrite(req)
	}

	return proxy, nil
}
