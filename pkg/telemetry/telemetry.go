package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/ghuser/ghproject/pkg/config"
)

// Shutdown flushes and stops all OTel providers
type Shutdown func(context.Context) error

// Setup initializes OTel trace and metric providers.
// A Prometheus reader is always registered so /metrics is always available.
// OTLP exporters are added only when cfg.OtelEndpoint is non-empty.
// Returns a shutdown function and an http.Handler for the /metrics endpoint.
func Setup(ctx context.Context, cfg *config.Config) (Shutdown, http.Handler, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel resource: %w", err)
	}

	// --- Traces ---
	var tp *sdktrace.TracerProvider
	if cfg.OtelEndpoint != "" {
		traceExp, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.OtelEndpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("otel trace exporter: %w", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp),
			sdktrace.WithResource(res),
		)
	} else {
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	}
	otel.SetTracerProvider(tp)

	// --- Metrics ---
	// Prometheus reader is always present so /metrics works in every environment.
	promExp, err := promexporter.New()
	if err != nil {
		return nil, nil, fmt.Errorf("prometheus exporter: %w", err)
	}

	mpOpts := []sdkmetric.Option{
		sdkmetric.WithReader(promExp),
		sdkmetric.WithResource(res),
	}

	// Also push to OTLP when an endpoint is configured.
	if cfg.OtelEndpoint != "" {
		metricExp, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(cfg.OtelEndpoint),
			otlpmetrichttp.WithInsecure(),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("otel metric exporter: %w", err)
		}
		mpOpts = append(mpOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)))
	}

	mp := sdkmetric.NewMeterProvider(mpOpts...)
	otel.SetMeterProvider(mp)

	shutdown := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return err
		}
		return mp.Shutdown(ctx)
	}

	return shutdown, promhttp.Handler(), nil
}
