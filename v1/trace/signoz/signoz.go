package signoz

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

type (
	signoz struct {
		serviceName  string
		collectorURL string
		insecure     string
	}

	Config struct {
		ServiceName  string
		CollectorURL string
		Insecure     string
	}

	KeyValue struct {
		Key   string
		Value string
	}

	Itf interface {
		InitTracer() func(context.Context) error
		CreateSpan(ctx context.Context, name string, err error, opts ...SpanTypeOption) (context.Context, trace.Span)
		EndSpan(span trace.Span)
		SetErrorSpan(span trace.Span, err error)
		SetAttributes(span trace.Span, attributes []KeyValue)
		AddEvent(span trace.Span, name string, attributes []KeyValue)
		TraceHttpRequest(ctx context.Context, token, userId, queryParam, payload string)
		TraceHttpResponse(ctx context.Context, code int, message string, data interface{}, errors interface{})
	}
)

var tracer trace.Tracer

func New(cfg Config) Itf {
	tracer = otel.Tracer(cfg.ServiceName)

	return &signoz{
		serviceName:  cfg.ServiceName,
		collectorURL: cfg.CollectorURL,
		insecure:     cfg.Insecure,
	}
}

func (s *signoz) InitTracer() func(context.Context) error {
	var secureOption otlptracegrpc.Option

	if strings.ToLower(s.insecure) == "false" || s.insecure == "0" || strings.ToLower(s.insecure) == "f" {
		secureOption = otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	} else {
		secureOption = otlptracegrpc.WithInsecure()
	}

	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			secureOption,
			otlptracegrpc.WithEndpoint(s.collectorURL),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", s.serviceName),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		log.Fatalf("Could not set resources: %v", err)
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resources),
		),
	)

	return exporter.Shutdown
}

func (s *signoz) CreateSpan(ctx context.Context, spanName string, err error, opts ...SpanTypeOption) (context.Context, trace.Span) {
	spanTypeConfig := spanTypeConfig{
		SpanType:         Unspecified,
		DatabasePlatform: "",
		ExternalURL:      "",
	}
	for _, opt := range opts {
		spanTypeConfig = opt.apply(spanTypeConfig)
	}

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(spanTypeMapper[spanTypeConfig.SpanType]))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	attributes := getSpanTypeAttributes(&spanTypeConfig)
	if attributes != nil {
		s.SetAttributes(span, attributes)
	}

	return ctx, span
}

func (s *signoz) EndSpan(span trace.Span) {
	span.End()
}

func (s *signoz) SetErrorSpan(span trace.Span, err error) {
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}

func (s *signoz) SetAttributes(span trace.Span, keyValue []KeyValue) {
	var kv []attribute.KeyValue

	for _, item := range keyValue {
		kv = append(kv, attribute.String(string(item.Key), string(item.Value)))
	}

	span.SetAttributes(kv...)
}

func (s *signoz) AddEvent(span trace.Span, name string, keyValue []KeyValue) {
	var (
		options    trace.EventOption
		attributes []attribute.KeyValue
	)

	for _, item := range keyValue {
		attributes = append(attributes, attribute.String(string(item.Key), string(item.Value)))
	}

	options = trace.WithAttributes(attributes...)
	span.AddEvent(name, options)
}

func (s *signoz) TraceHttpRequest(ctx context.Context, token, userId, queryParam, payload string) {
	span := trace.SpanFromContext(ctx)

	keyValueEvent := []KeyValue{
		{
			Key:   "Token",
			Value: token,
		},
		{
			Key:   "Query Param",
			Value: queryParam,
		},
		{
			Key:   "Payload",
			Value: payload,
		},
	}
	s.AddEvent(span, "Request", keyValueEvent)

	keyValueAttributes := []KeyValue{
		{
			Key:   "userId",
			Value: userId,
		},
	}
	s.SetAttributes(span, keyValueAttributes)
}

func (s *signoz) TraceHttpResponse(ctx context.Context, code int, message string, data interface{}, errors interface{}) {
	span := trace.SpanFromContext(ctx)

	dataString, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	errorsString, err := json.Marshal(errors)
	if err != nil {
		log.Fatal(err)
	}

	keyValue := []KeyValue{
		{
			Key:   "Code",
			Value: strconv.Itoa(code),
		},
		{
			Key:   "Message",
			Value: message,
		},
		{
			Key:   "Data",
			Value: string(dataString),
		},
		{
			Key:   "Errors",
			Value: string(errorsString),
		},
	}
	s.AddEvent(span, "Response", keyValue)
}
