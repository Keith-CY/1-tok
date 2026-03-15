package httputil

import "net/http"

// Middleware is a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain composes middlewares into a single wrapper.
// Middlewares are applied in order: first listed = outermost.
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// DefaultMiddleware returns the standard middleware stack.
func DefaultMiddleware() Middleware {
	return Chain(
		SecurityHeaders,
		RequestID,
		VersionHeader,
	)
}
