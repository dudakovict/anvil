// Package telemetry provides support for OpenTelemetry tracing and metrics.
package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

type Config struct {
	Enabled        bool
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	OTLPInsecure   bool
	SampleRatio    float64
}

// Init installs the global OTel providers and returns a shutdown function.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentNameKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("building otel resource: %w", err)
	}

	tp, err := initTracing(ctx, cfg, res)
	if err != nil {
		return nil, err
	}

	mp, err := initMetrics(ctx, cfg, res)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}

func initTracing(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	}

	if cfg.Enabled {
		expOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint)}
		if cfg.OTLPInsecure {
			expOpts = append(expOpts, otlptracegrpc.WithInsecure())
		}

		exporter, err := otlptracegrpc.New(ctx, expOpts...)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
		}

		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	return tp, nil
}

func initMetrics(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	opts := []sdkmetric.Option{sdkmetric.WithResource(res)}

	if cfg.Enabled {
		expOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint)}
		if cfg.OTLPInsecure {
			expOpts = append(expOpts, otlpmetricgrpc.WithInsecure())
		}

		exporter, err := otlpmetricgrpc.New(ctx, expOpts...)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
		}

		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)))
	}

	mp := sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(mp)

	if err := runtime.Start(); err != nil {
		return nil, fmt.Errorf("starting runtime metrics: %w", err)
	}

	return mp, nil
}
