// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package tracegen

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.8.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func SendOTLPTrace(ctx context.Context, cfg Config) error {
	endpointURL, err := url.Parse(cfg.apmServerURL)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint: %w", err)
	}
	switch endpointURL.Scheme {
	case "http":
		if endpointURL.Port() == "" {
			endpointURL.Host = net.JoinHostPort(endpointURL.Host, "80")
		}
	case "https":
		if endpointURL.Port() == "" {
			endpointURL.Host = net.JoinHostPort(endpointURL.Host, "443")
		}
	default:
		return fmt.Errorf("endpoint must be prefixed with http:// or https://")
	}

	otlpExporters, err := newOTLPExporters(ctx, endpointURL, cfg)
	if err != nil {
		return err
	}
	defer otlpExporters.cleanup(ctx)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(otlpExporters.trace),
		sdktrace.WithResource(
			resource.NewSchemaless(attribute.String("service.name", cfg.otlpServiceName)),
		),
	)
	// generateSpans returns ctx that contains trace context
	ctx, err = generateSpans(ctx, tracerProvider.Tracer("tracegen"))
	if err != nil {
		return err
	}
	if err := generateLogs(ctx, otlpExporters.log, cfg.otlpServiceName); err != nil {
		return err
	}

	// Shutdown, flushing all data to the server.
	if err := tracerProvider.Shutdown(ctx); err != nil {
		return err
	}
	return otlpExporters.cleanup(ctx)
}

func generateSpans(ctx context.Context, tracer trace.Tracer) (context.Context, error) {
	ctx, parent := tracer.Start(ctx, "parent", trace.WithSpanKind(trace.SpanKindServer))
	defer parent.End()

	_, child1 := tracer.Start(ctx, "child1")
	time.Sleep(10 * time.Millisecond)
	child1.AddEvent("an arbitrary event")
	child1.End()

	_, child2 := tracer.Start(ctx, "child2")
	time.Sleep(10 * time.Millisecond)
	child2.RecordError(errors.New("an exception occurred"))
	child2.End()

	return ctx, nil
}

func generateLogs(ctx context.Context, logger otlplogExporter, serviceName string) error {
	logs := plog.NewLogs()

	rl := logs.ResourceLogs().AppendEmpty()
	attribs := rl.Resource().Attributes()
	attribs.Insert(string(semconv.ServiceNameKey),
		pcommon.NewValueString(serviceName),
	)
	sl := rl.ScopeLogs().AppendEmpty().LogRecords()
	record := sl.AppendEmpty()
	record.Body().SetStringVal("sample body value")
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	record.SetSeverityNumber(plog.SeverityNumberFATAL)
	record.SetSeverityText("fatal")

	return logger.Export(ctx, logs)
}

type otlpExporters struct {
	cleanup func(context.Context) error
	trace   *otlptrace.Exporter
	log     otlplogExporter
}

func newOTLPExporters(ctx context.Context, endpointURL *url.URL, cfg Config) (*otlpExporters, error) {
	switch cfg.otlpProtocol {
	case "grpc":
		return newOTLPGRPCExporters(ctx, endpointURL, cfg)
	case "http/protobuf":
		return newOTLPHTTPExporters(ctx, endpointURL, cfg)
	default:
		return nil, fmt.Errorf("invalid protocol %q", cfg.otlpProtocol)
	}
}

func newOTLPGRPCExporters(ctx context.Context, endpointURL *url.URL, cfg Config) (*otlpExporters, error) {
	var transportCredentials credentials.TransportCredentials

	switch endpointURL.Scheme {
	case "http":
		// If http:// is specified, then use insecure (plaintext).
		transportCredentials = grpcinsecure.NewCredentials()
	case "https":
		transportCredentials = credentials.NewTLS(&tls.Config{InsecureSkipVerify: cfg.insecure})
	}

	grpcConn, err := grpc.DialContext(ctx, endpointURL.Host, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		return nil, err
	}
	cleanup := func(context.Context) error {
		return grpcConn.Close()
	}

	traceOptions := []otlptracegrpc.Option{otlptracegrpc.WithGRPCConn(grpcConn)}
	var logHeaders map[string]string
	headers := map[string]string{"Authorization": "ApiKey " + cfg.apiKey}
	traceOptions = append(traceOptions, otlptracegrpc.WithHeaders(headers))
	logHeaders = headers

	otlpTraceExporter, err := otlptracegrpc.New(ctx, traceOptions...)
	if err != nil {
		cleanup(ctx)
		return nil, err
	}
	cleanup = combineCleanup(otlpTraceExporter.Shutdown, cleanup)

	return &otlpExporters{
		cleanup: cleanup,
		trace:   otlpTraceExporter,
		log: &otlploggrpcExporter{
			client:  plogotlp.NewClient(grpcConn),
			headers: logHeaders,
		},
	}, nil
}

func newOTLPHTTPExporters(ctx context.Context, endpointURL *url.URL, cfg Config) (*otlpExporters, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: cfg.insecure}
	traceOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpointURL.Host),
		otlptracehttp.WithTLSClientConfig(tlsConfig),
	}
	if endpointURL.Scheme == "http" {
		traceOptions = append(traceOptions, otlptracehttp.WithInsecure())
	}

	headers := map[string]string{"Authorization": "ApiKey " + cfg.apiKey}
	traceOptions = append(traceOptions, otlptracehttp.WithHeaders(headers))

	cleanup := func(context.Context) error { return nil }

	otlpTraceExporter, err := otlptracehttp.New(ctx, traceOptions...)
	if err != nil {
		cleanup(ctx)
		return nil, err
	}
	cleanup = combineCleanup(otlpTraceExporter.Shutdown, cleanup)

	return &otlpExporters{
		cleanup: cleanup,
		trace:   otlpTraceExporter,
		log:     &otlploghttpExporter{},
	}, nil
}

func combineCleanup(a, b func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := a(ctx); err != nil {
			return err
		}
		return b(ctx)
	}
}

type otlplogExporter interface {
	Export(ctx context.Context, logs plog.Logs) error
}

// otlploggrpcExporter is a simple synchronous log exporter using GRPC
type otlploggrpcExporter struct {
	client  plogotlp.Client
	headers map[string]string
}

func (e *otlploggrpcExporter) Export(ctx context.Context, logs plog.Logs) error {
	req := plogotlp.NewRequestFromLogs(logs)
	md := metadata.New(e.headers)
	ctx = metadata.NewOutgoingContext(ctx, md)

	_, err := e.client.Export(ctx, req)
	if err != nil {
		return err
	}

	// TODO: parse response for error
	return nil
}

// otlploghttpExporter is a simple synchronous log exporter using protobuf over HTTP
type otlploghttpExporter struct {
}

func (e *otlploghttpExporter) Export(ctx context.Context, logs plog.Logs) error {
	// TODO: implement
	return errors.New("otlploghttpExporter isn't implemented")
}

func SetOTLPTracePropagator(ctx context.Context, traceparent string, tracestate string) context.Context {
	m := propagation.MapCarrier{}
	m.Set("traceparent", traceparent)
	m.Set("tracestate", tracestate)
	tc := propagation.TraceContext{}
	// Register the TraceContext propagator globally.
	otel.SetTextMapPropagator(tc)

	return tc.Extract(ctx, m)
}
