package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func NewSingleHost(upstream string, rewrite func(*http.Request)) http.Handler {
	target, err := url.Parse(upstream)
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		rewrite(req)
	}

	return proxy
}
