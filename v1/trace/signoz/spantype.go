package signoz

import (
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type SpanType int

const (
	Unspecified SpanType = 0
	Internal    SpanType = 1
	Server      SpanType = 2
	Client      SpanType = 3
	Producer    SpanType = 4
	Consumer    SpanType = 5
)

type DatabasePlatform string

const (
	Other   DatabasePlatform = "other_sql"
	MySQL   DatabasePlatform = "mysql"
	MariaDB DatabasePlatform = "mariadb"
	Redis   DatabasePlatform = "redis"
)

type ExternalURL string

type SpanTypeOption interface {
	apply(spanTypeConfig) spanTypeConfig
}

type spanTypeConfig struct {
	SpanType         SpanType
	DatabasePlatform DatabasePlatform
	ExternalURL      ExternalURL
}

type config func(spanTypeConfig) spanTypeConfig

func (fn config) apply(config spanTypeConfig) spanTypeConfig {
	return fn(config)
}

var (
	spanTypeMapper = map[SpanType]trace.SpanKind{
		Unspecified: trace.SpanKindUnspecified,
		Internal:    trace.SpanKindInternal,
		Server:      trace.SpanKindServer,
		Client:      trace.SpanKindClient,
		Producer:    trace.SpanKindProducer,
		Consumer:    trace.SpanKindConsumer,
	}
)

func DatabaseCalls(databasePlatform DatabasePlatform) SpanTypeOption {
	return config(func(config spanTypeConfig) spanTypeConfig {
		config.SpanType = Internal
		config.DatabasePlatform = databasePlatform
		return config
	})
}

func ExternalCalls(externalURL ExternalURL) SpanTypeOption {
	return config(func(config spanTypeConfig) spanTypeConfig {
		config.SpanType = Server
		config.ExternalURL = ExternalURL(externalURL)
		return config
	})
}

func getSpanTypeAttributes(spanTypeConfig *spanTypeConfig) []KeyValue {
	if spanTypeConfig == nil {
		return nil
	}

	attributes := []KeyValue{}
	switch spanTypeConfig.SpanType {
	case Internal:
		attributes = append(attributes, KeyValue{
			Key:   string(semconv.DBSystemMySQL.Key),
			Value: string(spanTypeConfig.DatabasePlatform),
		})
	case Server:
		attributes = append(attributes, KeyValue{
			Key:   string(semconv.HTTPURLKey),
			Value: string(spanTypeConfig.ExternalURL),
		})
	}

	return attributes
}
