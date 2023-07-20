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
	"errors"
	"fmt"
	"math"
	"os"

	"go.elastic.co/apm/v2"
)

type ConfigOption func(Config) Config
type Config struct {
	apmServerURL string
	apiKey       string
	sampleRate   float64
	traceID      apm.TraceID
	insecure     bool

	elasticAPMTracer *apm.Tracer

	otlpServiceName string
	otlpProtocol    string
}

func NewConfig(opts ...ConfigOption) Config {
	cfg := Config{
		sampleRate:   1.0,
		traceID:      NewRandomTraceID(),
		insecure:     false,
		otlpProtocol: "grpc",
	}
	for _, opt := range opts {
		cfg = opt(cfg)
	}

	cfg.configureEnv()

	return cfg
}

// WithSampleRate specifies the sample rate for the APM GO Agent
func WithSampleRate(r float64) ConfigOption {
	return func(c Config) Config {
		c.sampleRate = math.Round(r*10000) / 10000
		return c
	}
}

// WithAPMServerURL set APM Server URL (env value ELASTIC_APM_SERVER_URL)
func WithAPMServerURL(a string) ConfigOption {
	return func(c Config) Config {
		c.apmServerURL = a
		return c
	}
}

// WithAPIKey sets auth apiKey to communicate with APM Server
func WithAPIKey(k string) ConfigOption {
	return func(c Config) Config {
		c.apiKey = k
		return c
	}
}

// WithTraceID specifies the user defined traceID
func WithTraceID(t apm.TraceID) ConfigOption {
	return func(c Config) Config {
		c.traceID = t
		return c
	}
}

// WithInsecureConn skip the server's TLS certificate verification
func WithInsecureConn(b bool) ConfigOption {
	return func(c Config) Config {
		c.insecure = b
		return c
	}
}

// WithElasticAPMTracer sets tracer for the elastic GO Agent
// this config will be ignored when using SendOTLPTrace
func WithElasticAPMTracer(t *apm.Tracer) ConfigOption {
	return func(c Config) Config {
		c.elasticAPMTracer = t
		return c
	}
}

// WithOTLPServiceName specifies the service name that otlp will use
// this config will be ignored when using SendIntakeV2Trace
func WithOTLPServiceName(s string) ConfigOption {
	return func(c Config) Config {
		c.otlpServiceName = s
		return c
	}
}

// WithOTLPProtocol specifies OTLP transport protocol to one of: grpc (default), http/protobuf"
// this config will be ignored when using SendIntakeV2Trace
func WithOTLPProtocol(p string) ConfigOption {
	return func(c Config) Config {
		c.otlpProtocol = p
		return c
	}
}

func (cfg Config) Validate() error {
	if cfg.sampleRate < 0.0001 || cfg.sampleRate > 1.0 {
		return fmt.Errorf("invalid sample rate %f provided. allowed value: 0.0001 <= sample-rate <= 1.0", cfg.sampleRate)
	}

	if cfg.apiKey == "" || cfg.apmServerURL == "" {
		return errors.New("both API Key and APM Server URL must be configured")
	}
	return nil
}

// configureEnv parses or sets env configs to work with both Elastic GO Agent and OTLP library
func (cfg *Config) configureEnv() error {
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("ELASTIC_APM_API_KEY")
	} else {
		os.Setenv("ELASTIC_APM_API_KEY", cfg.apiKey)
	}

	if cfg.apmServerURL == "" {
		cfg.apmServerURL = os.Getenv("ELASTIC_APM_SERVER_URL")
	} else {
		os.Setenv("ELASTIC_APM_SERVER_URL", cfg.apmServerURL)
	}

	if cfg.insecure {
		os.Setenv("ELASTIC_APM_VERIFY_SERVER_CERT", "false")
	}
	return nil
}
