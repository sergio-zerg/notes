package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TraceExporter selects where spans are delivered.
type TraceExporter string

const (
	// ExporterStdout writes spans to stdout. Convenient locally, noisy in a
	// deployment where a real collector should be the destination.
	ExporterStdout TraceExporter = "stdout"
	// ExporterOTLP ships spans to an OTLP collector over gRPC.
	ExporterOTLP TraceExporter = "otlp"
	// ExporterNone discards spans while keeping the SDK installed, so the
	// instrumentation code keeps working without producing output.
	ExporterNone TraceExporter = "none"
)

// batchTimeout bounds how long a span may sit in the queue before export. It
// is shortened from the SDK default so that shutdown tests and short-lived
// processes still see their spans flushed.
const batchTimeout = 2 * time.Second

// TracerConfig describes how tracing should be set up.
type TracerConfig struct {
	ServiceName string
	Exporter    TraceExporter
	// OTLPEndpoint is the collector address, host:port, without a scheme.
	OTLPEndpoint string
	// SampleRatio is the fraction of root spans to record, within [0,1]. It is
	// ignored when the parent span has already made the decision.
	SampleRatio float64
}

// InitTracer installs a tracer provider and returns its shutdown function.
//
// The returned shutdown flushes pending spans, so callers must invoke it
// before the process exits or the last traces are lost.
func InitTracer(cfg TracerConfig) (func(context.Context) error, error) {
	if cfg.Exporter == ExporterNone {
		// Install a provider that records nothing rather than leaving the
		// global no-op, so a later SetTracerProvider still works predictably.
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.NeverSample()),
			sdktrace.WithResource(serviceResource(cfg.ServiceName)),
		)
		otel.SetTracerProvider(tp)
		return tp.Shutdown, nil
	}

	exporter, err := newExporter(cfg)
	if err != nil {
		return nil, err
	}

	// ParentBased keeps the sampling decision of an upstream service, so a
	// trace that is sampled at the edge is not silently truncated here.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(newSampler(cfg.SampleRatio))),
		// Batch rather than sync: a synchronous exporter adds its round trip
		// to the latency of the request that produced the span.
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(batchTimeout)),
		sdktrace.WithResource(serviceResource(cfg.ServiceName)),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func newExporter(cfg TracerConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case ExporterOTLP:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			// Plaintext: the collector is expected to sit on a trusted network
			// segment. Set OTEL_EXPORTER_OTLP_INSECURE=false and supply TLS
			// credentials before pointing this at a remote collector.
			otlptracegrpc.WithInsecure(),
		)
		exporter, err := otlptrace.New(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
		return exporter, nil

	case ExporterStdout:
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("create stdout trace exporter: %w", err)
		}
		return exporter, nil

	default:
		return nil, fmt.Errorf("unknown trace exporter %q: expected one of %q, %q, %q",
			cfg.Exporter, ExporterStdout, ExporterOTLP, ExporterNone)
	}
}

// newSampler maps a ratio to a sampler. AlwaysSample and NeverSample are used
// at the extremes because the ratio variants behave identically but do not
// make the intent as obvious in a debugger.
func newSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio <= 0:
		return sdktrace.NeverSample()
	case ratio >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.TraceIDRatioBased(ratio)
	}
}

func serviceResource(serviceName string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
	)
}

// GetTracer returns a tracer for the given name.
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
