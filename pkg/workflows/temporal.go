package workflows

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	temporallog "go.temporal.io/sdk/log"

	"github.com/ghuser/ghproject/pkg/logger"
)

// TemporalClient wraps the Temporal SDK client with project-level configuration.
type TemporalClient struct {
	Client    client.Client
	Namespace string
	log       logger.Logger
}

// NewTemporalClient initializes a Temporal client with OTel tracing integration.
// Call Close() when the application shuts down.
func NewTemporalClient(ctx context.Context, hostPort, namespace string, log logger.Logger) (*TemporalClient, error) {
	otelInterceptor, err := temporalotel.NewTracingInterceptor(temporalotel.TracerOptions{
		Tracer: otel.Tracer("temporal-client"),
	})
	if err != nil {
		return nil, fmt.Errorf("create temporal otel interceptor: %w", err)
	}

	c, err := client.Dial(client.Options{
		HostPort:     hostPort,
		Namespace:    namespace,
		Logger:       newTemporalLogger(log),
		Interceptors: []interceptor.ClientInterceptor{otelInterceptor},
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal server at %s: %w", hostPort, err)
	}

	log.Info("temporal client connected", "host_port", hostPort, "namespace", namespace)

	return &TemporalClient{
		Client:    c,
		Namespace: namespace,
		log:       log,
	}, nil
}

// Close gracefully shuts down the Temporal client connection.
func (tc *TemporalClient) Close() {
	tc.Client.Close()
	tc.log.Info("temporal client closed")
}

// temporalLogger adapts logger.Logger to Temporal's log.Logger interface.
type temporalLogger struct {
	log logger.Logger
}

func newTemporalLogger(log logger.Logger) temporallog.Logger {
	return &temporalLogger{log: log}
}

func (l *temporalLogger) Debug(msg string, keyvals ...interface{}) {
	l.log.Debug(msg, keyvals...)
}

func (l *temporalLogger) Info(msg string, keyvals ...interface{}) {
	l.log.Info(msg, keyvals...)
}

func (l *temporalLogger) Warn(msg string, keyvals ...interface{}) {
	l.log.Warn(msg, keyvals...)
}

func (l *temporalLogger) Error(msg string, keyvals ...interface{}) {
	l.log.Error(msg, keyvals...)
}
