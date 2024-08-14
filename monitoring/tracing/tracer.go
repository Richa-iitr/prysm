// Package tracing sets up jaeger as an opentracing tool
// for services in Prysm.
package tracing

import (
	"errors"

	"contrib.go.opencensus.io/exporter/jaeger"
	"github.com/prysmaticlabs/prysm/v5/runtime/version"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var log = logrus.WithField("prefix", "tracing")

// Setup creates and initializes a new tracing configuration..
func Setup(serviceName, processName, endpoint string, sampleFraction float64, enable bool) error {
	if !enable {
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.NeverSample()})
		return nil
	}

	if serviceName == "" {
		return errors.New("tracing service name cannot be empty")
	}

	trace.ApplyConfig(trace.Config{
		DefaultSampler:          trace.ProbabilitySampler(sampleFraction),
		MaxMessageEventsPerSpan: 500,
	})

	log.Infof("Starting Jaeger exporter endpoint at address = %s", endpoint)
	exporter, err := jaeger.NewExporter(jaeger.Options{
		CollectorEndpoint: endpoint,
		Process: jaeger.Process{
			ServiceName: serviceName,
			Tags: []jaeger.Tag{
				jaeger.StringTag("process_name", processName),
				jaeger.StringTag("version", version.Version()),
			},
		},
		BufferMaxCount: 10000,
		OnError: func(err error) {
			log.WithError(err).Error("Could not process span")
		},
	})
	if err != nil {
		return err
	}
	trace.RegisterExporter(exporter)

	return nil
}

// SetupOtel creates and initializes a new tracing configuration using OpenTelemetry..
func SetupOtel(serviceName, processName, endpoint string, sampleFraction float64, enable bool) error {
	if !enable {
		// If tracing is disabled, return immediately
		return nil
	}

	if serviceName == "" {
		return errors.New("tracing service name cannot be empty")
	}

	exporter, err := stdout.New(stdout.WithPrettyPrint())
	if err != nil {
		return err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("process_name", processName),
		)),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleFraction)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	log.Printf("Tracing enabled with endpoint: %s", endpoint)
	return nil
}
