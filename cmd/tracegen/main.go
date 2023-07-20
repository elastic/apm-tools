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

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"

	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/elastic/apm-tools/pkg/tracegen"
)

func main() {
	cfg := tracegen.Config{
		TraceID: tracegen.NewRandomTraceID(),
	}

	flag.StringVar(&cfg.APMServerURL, "server", "", "set APM Server URL (env value ELASTIC_APM_SERVER_URL)")
	flag.StringVar(&cfg.APIKey, "api-key", "", "set APM API key for auth (env value ELASTIC_APM_API_KEY)")
	flag.Float64Var(&cfg.SampleRate, "sample-rate", 1.0, "set the sample rate. allowed value: min: 0.0001, max: 1.000")
	flag.StringVar(&cfg.OTLPProtocol, "otlp-protocol", "grpc", "set OTLP transport protocol to one of: grpc (default), http/protobuf")
	flag.BoolVar(&cfg.Insecure, "insecure", false, "skip the server's TLS certificate verification")

	flag.Parse()
	// get or set env values for GO agent and otel agent
	err := configureEnv(&cfg)
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	if cfg.SampleRate < 0.0001 || cfg.SampleRate > 1.0 {
		log.Fatalf("invalid sample rate %f provided. allowed value: 0.0001 <= sample-rate <= 1.0", cfg.SampleRate)
	}
	cfg.SampleRate = math.Round(cfg.SampleRate*10000) / 10000

	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()

	if err := Main(ctx, cfg); err != nil {
		log.Fatal("error sending distributed tracing data", zap.Error(err))
	}
}

func Main(ctx context.Context, cfg tracegen.Config) error {
	apmTracer, err := apm.NewTracer(getUniqueServiceName("service", "intake"), "0.0.1")
	if err != nil {
		return errors.New("failed to instantiate apm tracer")
	}

	cfg.ElasticAPMTracer = apmTracer
	cfg.OTLPServiceName = getUniqueServiceName("service", "otlp")

	return tracegen.SendDistributedTrace(ctx, cfg)
}

// configureEnv parses or sets env configs to work with both Elastic GO Agent and OTLP library
func configureEnv(cfg *tracegen.Config) error {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("ELASTIC_APM_API_KEY")
	} else {
		os.Setenv("ELASTIC_APM_API_KEY", cfg.APIKey)
	}

	if cfg.APMServerURL == "" {
		cfg.APMServerURL = os.Getenv("ELASTIC_APM_SERVER_URL")
	} else {
		os.Setenv("ELASTIC_APM_SERVER_URL", cfg.APMServerURL)
	}

	if cfg.APIKey == "" || cfg.APMServerURL == "" {
		return errors.New("both API Key and APM Server URL must be configured")
	}

	if cfg.Insecure {
		os.Setenv("ELASTIC_APM_VERIFY_SERVER_CERT", "false")
	}
	return nil
}

func getUniqueServiceName(prefix string, suffix string) string {
	uniqueName := suffixString(suffix)
	return prefix + "-" + uniqueName
}

func suffixString(s string) string {
	const letter = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return fmt.Sprintf("%s-%s", s, string(b))
}
