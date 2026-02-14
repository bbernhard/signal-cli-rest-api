package utils

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	log "github.com/sirupsen/logrus"
)

// TelemetryShutdown is a function type for shutting down telemetry
type TelemetryShutdown func(context.Context) error

// SetupTelemetry initializes OpenTelemetry with autoexport trace exporter
// It returns a shutdown function that should be called on application exit
// Telemetry is disabled by default and must be enabled via OTEL_EXPORTER_OTLP_ENDPOINT
// or other OTEL environment variables
func SetupTelemetry(ctx context.Context) (TelemetryShutdown, error) {
	// Telemetry is opt-in - only enable if OTEL environment variables are set
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" &&
	   os.Getenv("OTEL_TRACES_EXPORTER") == "" {
		log.Debug("OpenTelemetry is disabled (no exporter configured)")
		return func(context.Context) error { return nil }, nil
	}

	// Create resource - will automatically use OTEL_SERVICE_NAME and other env vars
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, err
	}

	// Set up trace exporter using autoexport
	traceExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}

	// Create trace provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	// Set up propagators
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	log.Info("OpenTelemetry tracing initialized successfully")

	// Return shutdown function
	return tracerProvider.Shutdown, nil
}

// GracefulShutdown handles graceful shutdown of telemetry
func GracefulShutdown(shutdown TelemetryShutdown) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := shutdown(ctx); err != nil {
		log.Error("Failed to shutdown telemetry: ", err)
	} else {
		log.Info("Telemetry shutdown successfully")
	}
}
