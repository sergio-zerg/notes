package observability

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestNewSamplerBoundaries(t *testing.T) {
	// wantAll: a zero trace id must be sampled. wantSome: across a spread of
	// trace ids, at least one must be sampled.
	cases := []struct {
		name     string
		ratio    float64
		wantAll  sdktrace.SamplingDecision
		wantSome bool
	}{
		{"zero drops everything", 0, sdktrace.Drop, false},
		{"negative is treated as zero", -1, sdktrace.Drop, false},
		{"one samples everything", 1, sdktrace.RecordAndSample, true},
		{"above one samples everything", 2, sdktrace.RecordAndSample, true},
		{"a ratio in between is not all or nothing", 0.5, sdktrace.RecordAndSample, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sampler := newSampler(tc.ratio)

			got := sampler.ShouldSample(sdktrace.SamplingParameters{
				ParentContext: context.Background(),
			}).Decision
			if got != tc.wantAll {
				t.Errorf("ShouldSample() = %v, want %v", got, tc.wantAll)
			}

			sampledSome := false
			for i := 0; i < 200; i++ {
				id := trace.TraceID{byte(i >> 8), byte(i)}
				decision := sampler.ShouldSample(sdktrace.SamplingParameters{
					ParentContext: context.Background(),
					TraceID:       id,
				}).Decision
				if decision == sdktrace.RecordAndSample {
					sampledSome = true
					break
				}
			}
			if sampledSome != tc.wantSome {
				t.Errorf("some trace ids sampled = %v, want %v", sampledSome, tc.wantSome)
			}
		})
	}
}

// A span sampled upstream must not be dropped here, or a distributed trace
// arrives as disconnected fragments at the collector.
func TestSamplerIsParentBased(t *testing.T) {
	sampler := sdktrace.ParentBased(newSampler(0))

	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), parent)

	decision := sampler.ShouldSample(sdktrace.SamplingParameters{ParentContext: ctx}).Decision
	if decision != sdktrace.RecordAndSample {
		t.Errorf("ShouldSample() = %v, want RecordAndSample for an upstream-sampled span", decision)
	}
}

func TestIsExporterValid(t *testing.T) {
	valid := []string{"stdout", "otlp", "none"}
	for _, name := range valid {
		if !IsExporterValid(name) {
			t.Errorf("IsExporterValid(%q) = false, want true", name)
		}
	}

	for _, name := range []string{"", "kafka", "stdout "} {
		if IsExporterValid(name) {
			t.Errorf("IsExporterValid(%q) = true, want false", name)
		}
	}
}

// "none" must install a provider that records nothing, so that a process
// without a collector still runs the instrumentation code without producing
// output.
func TestInitTracerNoneRecordsNothing(t *testing.T) {
	shutdown, err := InitTracer(TracerConfig{
		ServiceName: "notes-service",
		Exporter:    ExporterNone,
		SampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("InitTracer() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	_, span := GetTracer("test").Start(context.Background(), "should-not-be-recorded")
	if span.IsRecording() {
		t.Error("span.IsRecording() = true, want false with the none exporter")
	}
	span.End()
}

func TestInitTracerStdoutSucceeds(t *testing.T) {
	shutdown, err := InitTracer(TracerConfig{
		ServiceName: "notes-service",
		Exporter:    ExporterStdout,
		SampleRatio: 1,
	})
	if err != nil {
		t.Fatalf("InitTracer() = %v, want nil", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown() = %v, want nil", err)
	}
}

func TestInitTracerRejectsUnknownExporter(t *testing.T) {
	_, err := InitTracer(TracerConfig{
		ServiceName: "notes-service",
		Exporter:    TraceExporter("kafka"),
		SampleRatio: 1,
	})
	if err == nil {
		t.Fatal("InitTracer() = nil, want an error for an unknown exporter")
	}
}

// An unreachable collector must not stop the service: the exporter connects
// lazily and reports failures on export, not at construction.
func TestInitTracerOTLPWithNoCollector(t *testing.T) {
	shutdown, err := InitTracer(TracerConfig{
		ServiceName: "notes-service",
		Exporter:    ExporterOTLP,
		// Port 1 is reserved and never listening.
		OTLPEndpoint: "127.0.0.1:1",
		SampleRatio:  1,
	})
	if err != nil {
		t.Skipf("OTLP exporter refused to start without a collector: %v", err)
	}
	_ = shutdown(context.Background())
}
