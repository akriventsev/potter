// Package metrics предоставляет функции для настройки системы метрик.
package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// MetricsConfig конфигурация метрик
type MetricsConfig struct {
	ExporterType    string
	PrometheusPort  int
	OTLPEndpoint    string
	JaegerEndpoint  string
	SamplingRate    float64
	ResourceAttrs   map[string]string
}

// SetupMetrics настраивает экспорт метрик
func SetupMetrics(config *MetricsConfig) (*metric.MeterProvider, error) {
	if config == nil {
		config = &MetricsConfig{
			ExporterType: "prometheus",
			SamplingRate: 1.0,
		}
	}

	var reader metric.Reader
	var err error

	switch config.ExporterType {
	case "prometheus":
		reader, err = setupPrometheusExporter()
	case "otlp":
		reader, err = setupOTLPExporter(config.OTLPEndpoint)
	case "jaeger":
		reader, err = setupJaegerExporter(config.JaegerEndpoint)
	default:
		return nil, fmt.Errorf("unknown exporter type: %s", config.ExporterType)
	}

	if err != nil {
		return nil, err
	}

	// Создаем resource attributes
	res, err := resource.New(context.Background(),
		resource.WithAttributes(buildResourceAttributes(config.ResourceAttrs)...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
	)

	otel.SetMeterProvider(provider)

	return provider, nil
}

// setupPrometheusExporter настраивает Prometheus exporter
func setupPrometheusExporter() (metric.Reader, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}
	return exporter, nil
}

// setupOTLPExporter настраивает OTLP exporter
// NOTE: OTLP exporter для метрик не реализован в текущей версии.
// Используйте Prometheus exporter или дождитесь будущих обновлений.
func setupOTLPExporter(endpoint string) (metric.Reader, error) {
	return nil, fmt.Errorf("OTLP exporter for metrics is not implemented in this version. Use Prometheus exporter or wait for future updates")
}

// setupJaegerExporter настраивает Jaeger exporter
// NOTE: Jaeger exporter для метрик не реализован в текущей версии.
// Jaeger обычно используется для трейсинга, а не для метрик.
// Используйте Prometheus exporter для метрик или OTLP для трейсинга.
func setupJaegerExporter(endpoint string) (metric.Reader, error) {
	return nil, fmt.Errorf("jaeger exporter for metrics is not implemented. Jaeger is typically used for tracing, not metrics. Use Prometheus exporter for metrics")
}

// buildResourceAttributes строит resource attributes
func buildResourceAttributes(attrs map[string]string) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		result = append(result, attribute.String(k, v))
	}
	return result
}

// ShutdownMetrics корректно завершает работу метрик
func ShutdownMetrics(ctx context.Context, provider *metric.MeterProvider) error {
	if provider == nil {
		return nil
	}

	return provider.Shutdown(ctx)
}

