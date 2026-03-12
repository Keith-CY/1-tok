package observability

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

type Config struct {
	Service          string  `json:"service"`
	DSN              string  `json:"dsn"`
	Environment      string  `json:"environment"`
	Release          string  `json:"release"`
	SampleRate       float64 `json:"sampleRate"`
	TracesSampleRate float64 `json:"tracesSampleRate"`
}

type RequestTags struct {
	Route   string
	OrgID   string
	UserID  string
	OrderID string
	RFQID   string
	Subject string
}

type tagBagKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func ConfigFromEnv(service string) Config {
	return Config{
		Service:          strings.TrimSpace(service),
		DSN:              strings.TrimSpace(os.Getenv("SENTRY_DSN")),
		Environment:      strings.TrimSpace(os.Getenv("SENTRY_ENVIRONMENT")),
		Release:          strings.TrimSpace(os.Getenv("SENTRY_RELEASE")),
		SampleRate:       envFloat("SENTRY_SAMPLE_RATE", 1),
		TracesSampleRate: envFloat("SENTRY_TRACES_SAMPLE_RATE", 0),
	}
}

func InitFromEnv(service string) (func(time.Duration) bool, error) {
	return Init(ConfigFromEnv(service))
}

func Init(cfg Config) (func(time.Duration) bool, error) {
	clientOptions := sentry.ClientOptions{
		Dsn:              cfg.DSN,
		AttachStacktrace: true,
		ServerName:       cfg.Service,
		Environment:      fallback(cfg.Environment, "development"),
		Release:          cfg.Release,
		SampleRate:       cfg.SampleRate,
		TracesSampleRate: cfg.TracesSampleRate,
	}
	if err := sentry.Init(clientOptions); err != nil {
		return nil, err
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("service", cfg.Service)
		scope.SetTag("environment", fallback(cfg.Environment, "development"))
		if strings.TrimSpace(cfg.Release) != "" {
			scope.SetTag("release", cfg.Release)
		}
	})

	return Flush, nil
}

func Flush(timeout time.Duration) bool {
	return sentry.Flush(timeout)
}

func WrapHTTP(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub := sentry.CurrentHub().Clone()
		bag := &RequestTags{Route: r.URL.Path}
		ctx := context.WithValue(r.Context(), tagBagKey{}, bag)
		ctx = sentry.SetHubOnContext(ctx, hub)
		r = r.WithContext(ctx)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if recovered := recover(); recovered != nil {
				hub.WithScope(func(scope *sentry.Scope) {
					applyScope(scope, service, r, bag)
					scope.SetExtra("panic", fmt.Sprint(recovered))
					hub.RecoverWithContext(r.Context(), recovered)
				})
				if recorder.status < http.StatusBadRequest {
					http.Error(recorder, "internal server error", http.StatusInternalServerError)
				}
				return
			}

			if recorder.status >= http.StatusInternalServerError {
				hub.WithScope(func(scope *sentry.Scope) {
					applyScope(scope, service, r, bag)
					scope.SetLevel(sentry.LevelError)
					hub.CaptureMessage(fmt.Sprintf("http %d on %s", recorder.status, r.URL.Path))
				})
			}
		}()

		next.ServeHTTP(recorder, r)
	})
}

func WithRequestTags(r *http.Request, tags RequestTags) *http.Request {
	if r == nil {
		return r
	}
	bag := requestTagsFromContext(r.Context())
	if bag == nil {
		bag = &RequestTags{}
		r = r.WithContext(context.WithValue(r.Context(), tagBagKey{}, bag))
	}
	if tags.Route != "" {
		bag.Route = tags.Route
	}
	if tags.OrgID != "" {
		bag.OrgID = tags.OrgID
	}
	if tags.UserID != "" {
		bag.UserID = tags.UserID
	}
	if tags.OrderID != "" {
		bag.OrderID = tags.OrderID
	}
	if tags.RFQID != "" {
		bag.RFQID = tags.RFQID
	}
	if tags.Subject != "" {
		bag.Subject = tags.Subject
	}
	return r
}

func CaptureMessage(ctx context.Context, message string) {
	captureWithScope(ctx, func(hub *sentry.Hub) {
		hub.CaptureMessage(message)
	})
}

func CaptureError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	captureWithScope(ctx, func(hub *sentry.Hub) {
		hub.CaptureException(err)
	})
}

func captureWithScope(ctx context.Context, capture func(*sentry.Hub)) {
	if ctx == nil {
		ctx = context.Background()
	}
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}
	if hub == nil {
		return
	}
	hub.WithScope(func(scope *sentry.Scope) {
		if bag := requestTagsFromContext(ctx); bag != nil {
			applyTags(scope, *bag)
		}
		capture(hub)
	})
}

func requestTagsFromContext(ctx context.Context) *RequestTags {
	if ctx == nil {
		return nil
	}
	bag, _ := ctx.Value(tagBagKey{}).(*RequestTags)
	return bag
}

func applyScope(scope *sentry.Scope, service string, r *http.Request, bag *RequestTags) {
	if scope == nil {
		return
	}
	scope.SetRequest(r)
	if service != "" {
		scope.SetTag("service", service)
	}
	if bag != nil {
		applyTags(scope, *bag)
	}
}

func applyTags(scope *sentry.Scope, tags RequestTags) {
	if tags.Route != "" {
		scope.SetTag("route", tags.Route)
	}
	if tags.OrgID != "" {
		scope.SetTag("org_id", tags.OrgID)
	}
	if tags.UserID != "" {
		scope.SetTag("user_id", tags.UserID)
	}
	if tags.OrderID != "" {
		scope.SetTag("order_id", tags.OrderID)
	}
	if tags.RFQID != "" {
		scope.SetTag("rfq_id", tags.RFQID)
	}
	if tags.Subject != "" {
		scope.SetTag("subject", tags.Subject)
	}
}

func envFloat(key string, fallbackValue float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallbackValue
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallbackValue
	}
	return parsed
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
