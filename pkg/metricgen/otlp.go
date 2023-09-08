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

package metricgen

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
)

// SendOTLP sends specific metrics to the configured Elastic APM OTLP intake.
//
// Metrics are sent via the specified protocol.
//
// Metrics sent are:
// - otlp(float64, value=1.0)
func SendOTLP(ctx context.Context, opts ...ConfigOption) (EventStats, error) {
	cfg := newConfig(opts...)
	if err := cfg.Validate(); err != nil {
		return EventStats{}, fmt.Errorf("cannot validate OTLP Metrics configuration: %w", err)
	}

	var exporter sdkmetric.Exporter
	switch cfg.otlpProtocol {
	case grpcOTLPProtocol:
		e, cleanup, err := newOTLPMetricGRPCExporter(ctx, cfg)
		if err != nil {
			return EventStats{}, err
		}
		defer cleanup()

		exporter = e
	case httpOTLPProtocol:
		e, err := newOTLPMetricHTTPExporter(ctx, cfg)
		if err != nil {
			return EventStats{}, err
		}

		exporter = e
	}
	defer exporter.Shutdown(ctx)

	resource := resource.NewSchemaless(
		attribute.String("service.name", cfg.otlpServiceName),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(resource),
	)

	stats := EventStats{}
	if err := generateMetrics(mp.Meter("metricgen"), &stats); err != nil {
		return stats, fmt.Errorf("cannot generate metrics: %w", err)
	}

	if err := mp.Shutdown(ctx); err != nil {
		return EventStats{}, fmt.Errorf("cannot shut down meter provider: %w", err)
	}

	return stats, nil
}

func generateMetrics(m metric.Meter, stats *EventStats) error {
	counter, _ := m.Float64Counter("otlp")
	counter.Add(context.Background(), 1)
	stats.Add(1)

	return nil
}

func newOTLPMetricHTTPExporter(ctx context.Context, cfg config) (*otlpmetrichttp.Exporter, error) {
	endpoint, err := otlpEndpoint(cfg.apmServerURL)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: !cfg.verifyServerCert}
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpoint.Host),
		otlpmetrichttp.WithTLSClientConfig(tlsConfig),
	}
	if endpoint.Scheme == "http" {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	headers := map[string]string{"Authorization": "ApiKey " + cfg.apiKey}
	opts = append(opts, otlpmetrichttp.WithHeaders(headers))

	return otlpmetrichttp.New(ctx, opts...)
}

func otlpEndpoint(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return &url.URL{}, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	switch u.Scheme {
	case "http":
		if u.Port() == "" {
			u.Host = net.JoinHostPort(u.Host, "80")
		}
	case "https":
		if u.Port() == "" {
			u.Host = net.JoinHostPort(u.Host, "443")
		}
	default:
		return &url.URL{}, fmt.Errorf("endpoint must be prefixed with http:// or https://")
	}

	return u, nil
}

func newOTLPMetricGRPCExporter(ctx context.Context, cfg config) (*otlpmetricgrpc.Exporter, func(), error) {
	endpoint, err := otlpEndpoint(cfg.apmServerURL)
	if err != nil {
		return nil, func() {}, err
	}

	var transportCredentials credentials.TransportCredentials
	switch endpoint.Scheme {
	case "http":
		// If http:// is specified, then use insecure (plaintext).
		transportCredentials = grpcinsecure.NewCredentials()
	case "https":
		transportCredentials = credentials.NewTLS(&tls.Config{InsecureSkipVerify: !cfg.verifyServerCert})
	}

	grpcConn, err := grpc.DialContext(
		ctx, endpoint.Host,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")),
	)
	cleanup := func() { grpcConn.Close() }
	if err != nil {
		return nil, cleanup, fmt.Errorf("cannot create grpc dial context: %w", err)
	}

	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithGRPCConn(grpcConn)}
	headers := map[string]string{"Authorization": "ApiKey " + cfg.apiKey}
	opts = append(opts, otlpmetricgrpc.WithHeaders(headers))

	e, err := otlpmetricgrpc.New(ctx, opts...)
	return e, cleanup, err
}
