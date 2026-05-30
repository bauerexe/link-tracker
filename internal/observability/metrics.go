package observability

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ScopeDatabase       = "database"
	ScopeLLMAgent       = "llm_agent"
	ScopeExternalSource = "external_source"
	ScopeKafka          = "kafka"

	ScopeScrapperSyncAPI  = "scrapper_sync_api"
	ScopeScrapperAsyncAPI = "scrapper_async_api"

	unknownLabel              = "unknown"
	microsecondsInMillisecond = 1000
	clientClosedRequestStatus = 499
)

var durationBucketsMS = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000}

var (
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_ms_total",
			Help:    "Duration of Scrapper component operations in milliseconds.",
			Buckets: durationBucketsMS,
		},
		[]string{"scope", "scope_type"},
	)

	apiRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of incoming Scrapper API requests.",
		},
		[]string{"source", "method", "path", "status"},
	)

	apiRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_ms_total",
			Help:    "Duration of incoming Scrapper API requests in milliseconds.",
			Buckets: durationBucketsMS,
		},
		[]string{"source", "method", "path", "status"},
	)

	commandRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "command_requests_total",
			Help: "Total number of processed bot commands and user requests.",
		},
		[]string{"command"},
	)

	commandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "command_duration_ms_total",
			Help:    "Duration of Bot calls to Scrapper in milliseconds.",
			Buckets: durationBucketsMS,
		},
		[]string{"scope", "scope_type"},
	)

	sentNotifications = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sent_notification_total",
			Help: "Total number of successfully sent user notifications.",
		},
	)
)

func ObserveRequestDuration(scope, scopeType string, started time.Time) {
	requestDuration.WithLabelValues(label(scope), label(scopeType)).
		Observe(millisecondsSince(started))
}

func ObserveCommandDuration(scope, scopeType string, started time.Time) {
	commandDuration.WithLabelValues(label(scope), label(scopeType)).
		Observe(millisecondsSince(started))
}

func IncCommandRequest(command string) {
	commandRequests.WithLabelValues(NormalizeCommand(command)).Inc()
}

func IncSentNotification() {
	sentNotifications.Inc()
}

func InstrumentHTTP(source string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		statusCode := recorder.status
		path := NormalizeHTTPPath(r.URL.Path)
		statusLabel := strconv.Itoa(statusCode)

		apiRequests.WithLabelValues(source, r.Method, path, statusLabel).Inc()
		apiRequestDuration.WithLabelValues(source, r.Method, path, statusLabel).
			Observe(millisecondsSince(started))
	})
}

func UnaryServerInterceptor(source string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()

		resp, err := handler(ctx, req)

		statusCode := grpcCodeToHTTPStatus(status.Code(err))
		statusLabel := strconv.Itoa(statusCode)
		method := "grpc"
		path := label(info.FullMethod)

		apiRequests.WithLabelValues(source, method, path, statusLabel).Inc()
		apiRequestDuration.WithLabelValues(source, method, path, statusLabel).
			Observe(millisecondsSince(started))

		return resp, err
	}
}

func NormalizeCommand(command string) string {
	command = strings.TrimSpace(strings.TrimPrefix(command, "/"))
	switch command {
	case "start", "help", "track", "untrack", "list":
		return command
	default:
		return "unknown"
	}
}

func NormalizeHTTPPath(path string) string {
	if path == "" {
		return "/"
	}

	parts := strings.Split(path, "/")
	for i, part := range parts {
		if isNumeric(part) {
			parts[i] = "{id}"
		}
	}

	return strings.Join(parts, "/")
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func millisecondsSince(started time.Time) float64 {
	return float64(time.Since(started).Microseconds()) / microsecondsInMillisecond
}

func label(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return unknownLabel
	}
	return value
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func grpcCodeToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return clientClosedRequestStatus
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unknown, codes.Internal, codes.DataLoss:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
