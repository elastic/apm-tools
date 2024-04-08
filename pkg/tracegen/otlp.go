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
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// SendOTLPTrace sends spans, error and logs to the configured APM Server
// If distributed tracing is needed, you might want to set up the propagator
// using SetOTLPTracePropagator function before calling this function
func SendOTLPTrace(ctx context.Context, cfg Config) (EventStats, error) {
	if err := cfg.validate(); err != nil {
		return EventStats{}, err
	}

	endpointURL, err := url.Parse(cfg.apmServerURL)
	if err != nil {
		return EventStats{}, fmt.Errorf("failed to parse endpoint: %w", err)
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
		return EventStats{}, fmt.Errorf("endpoint must be prefixed with http:// or https://")
	}

	otlpExporters, err := newOTLPExporters(ctx, endpointURL, cfg)
	if err != nil {
		return EventStats{}, err
	}
	defer otlpExporters.cleanup(ctx)

	resource := resource.NewSchemaless(
		attribute.String("service.name", cfg.otlpServiceName),
	)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(otlpExporters.trace),
		sdktrace.WithResource(resource),
	)

	// generateSpans returns ctx that contains trace context
	var stats EventStats
	ctx, err = generateSpans(ctx, tracerProvider.Tracer("tracegen"), &stats)
	if err != nil {
		return EventStats{}, err
	}
	if err := generateLogs(ctx, otlpExporters.log, resource, &stats); err != nil {
		return EventStats{}, err
	}

	// Shutdown, flushing all data to the server.
	if err := tracerProvider.Shutdown(ctx); err != nil {
		return EventStats{}, err
	}
	if err := otlpExporters.cleanup(ctx); err != nil {
		return EventStats{}, err
	}
	return stats, nil
}

func generateSpans(ctx context.Context, tracer trace.Tracer, stats *EventStats) (context.Context, error) {
	now := time.Now()
	ctx, parent := tracer.Start(ctx,
		"parent",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithTimestamp(now),
	)
	defer parent.End(trace.WithTimestamp(now.Add(time.Millisecond * 1500)))
	stats.SpansSent++

	_, child1 := tracer.Start(ctx, "child1", trace.WithTimestamp(now.Add(time.Millisecond*500)))
	time.Sleep(10 * time.Millisecond)
	child1.AddEvent("an arbitrary event")
	child1.End(trace.WithTimestamp(now.Add(time.Second * 1)))
	stats.SpansSent++
	stats.LogsSent++ // span event is captured as a log

	_, child2 := tracer.Start(ctx, "child2", trace.WithTimestamp(now.Add(time.Millisecond*600)))
	time.Sleep(10 * time.Millisecond)
	child2.RecordError(errors.New("an exception occurred"))
	child2.End(trace.WithTimestamp(now.Add(time.Millisecond * 1300)))
	stats.SpansSent++
	stats.ExceptionsSent++ // error captured as an error/exception log event

	return ctx, nil
}

func generateLogs(ctx context.Context, logger otlplogExporter, res *resource.Resource, stats *EventStats) error {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	attribs := rl.Resource().Attributes()
	for iter := res.Iter(); iter.Next(); {
		kv := iter.Attribute()
		switch typ := kv.Value.Type(); typ {
		case attribute.STRING:
			attribs.PutStr(string(kv.Key), kv.Value.AsString())
		default:
			panic(fmt.Errorf("unhandled attribute type %q", typ))
		}
	}

	sl := rl.ScopeLogs().AppendEmpty().LogRecords()
	record := sl.AppendEmpty()
	record.Body().SetStr("sample body value")
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	record.SetSeverityNumber(plog.SeverityNumberFatal)
	record.SetSeverityText("fatal")
	stats.LogsSent++
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

	grpcConn, err := grpc.NewClient(
		endpointURL.Host,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")),
	)
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
			client:  plogotlp.NewGRPCClient(grpcConn),
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
	client  plogotlp.GRPCClient
	headers map[string]string
}

func (e *otlploggrpcExporter) Export(ctx context.Context, logs plog.Logs) error {
	req := plogotlp.NewExportRequestFromLogs(logs)
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
