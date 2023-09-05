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
	"errors"
	"fmt"
)

type ConfigOption func(*Config)

type Config struct {
	// apiKey holds an Elasticsearch API key.
	apiKey string
	// apmServerURL holdes the Elasticsearch APM server URL endpoint.
	apmServerURL string
	// verifyServerCert determines if endpoint TLS certificates will be validated.
	verifyServerCert bool

	// apmServiceName holds the service name sent with Elastic APM metrics.
	apmServiceName string
	// otlpServiceName holds the service name sent with OTLP metrics.
	otlpServiceName string
	// otlpProtocol specifies the OTLP protocol to use for sending metrics.
	// Valid values are: grpc, http/protobuf.
	otlpProtocol string
}

const (
	grpcOTLPProtocol = "grpc"
	httpOTLPProtocol = "http/protobuf"
)

func (cfg Config) Validate() error {
	var errs []error
	if cfg.apmServiceName == "" && cfg.otlpServiceName == "" {
		errs = append(errs, errors.New("both APM service name and OTLP service name cannot be empty"))
	}

	if cfg.apmServerURL == "" {
		errs = append(errs, errors.New("APM server URL cannot be empty"))
	}
	if cfg.apiKey == "" {
		errs = append(errs, errors.New("API Key cannot be empty"))
	}

	switch cfg.otlpProtocol {
	case httpOTLPProtocol, grpcOTLPProtocol:
	default:
		errs = append(errs, fmt.Errorf("unknown otlp protocol: %s", cfg.otlpProtocol))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func NewConfig(opts ...ConfigOption) Config {
	cfg := Config{
		otlpProtocol: "grpc",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

func WithAPIKey(s string) ConfigOption {
	return func(c *Config) {
		c.apiKey = s
	}
}

func WithAPMServerURL(s string) ConfigOption {
	return func(c *Config) {
		c.apmServerURL = s
	}
}

func WithVerifyServerCert(b bool) ConfigOption {
	return func(c *Config) {
		c.verifyServerCert = b
	}
}

// WithElasticAPMServiceName specifies the service name that
// the Elastic APM agent will use.
//
// This config will be ignored when using SendOTLPTrace.
func WithElasticAPMServiceName(s string) ConfigOption {
	return func(c *Config) {
		c.apmServiceName = s
	}
}

// WithOTLPServiceName specifies the service name that the
// OpenTelemetry SDK will use.
//
// This config will be ignored when using SendIntakeV2Trace.
func WithOTLPServiceName(s string) ConfigOption {
	return func(c *Config) {
		c.otlpServiceName = s
	}
}

// WithOTLPProtocol specifies OTLP transport protocol to one of:
// grpc (default), http/protobuf.
//
// This config will be ignored when using SendIntakeV2Trace
func WithOTLPProtocol(p string) ConfigOption {
	return func(c *Config) {
		c.otlpProtocol = p
	}
}
