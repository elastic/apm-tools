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
	"log"
	"math"
	"os"
	"os/signal"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapgrpc"
	"google.golang.org/grpc/grpclog"

	"github.com/elastic/apm-tools/pkg/tracegen"
)

func main() {
	var cfg tracegen.Config

	flag.StringVar(&cfg.APMServerURL, "server", "", "set APM Server URL (env value ELASTIC_APM_SERVER_URL)")
	flag.StringVar(&cfg.APIKey, "api-key", "", "set APM API key for auth (env value ELASTIC_APM_API_KEY)")
	flag.Float64Var(&cfg.SampleRate, "sample-rate", 1.0, "set the sample rate. allowed value: min: 0.0001, max: 1.000")
	flag.StringVar(&cfg.OTLPProtocol, "otlp-protocol", "grpc", "set OTLP transport protocol to one of: grpc (default), http/protobuf")
	flag.BoolVar(&cfg.Insecure, "insecure", false, "skip the server's TLS certificate verification")
	logLevel := zap.LevelFlag(
		"loglevel", zapcore.InfoLevel,
		"set log level to one of: DEBUG, INFO (default), WARN, ERROR, DPANIC, PANIC, FATAL",
	)

	flag.Parse()
	// get or set env values for GO agent and otel agent
	err := configureEnv(&cfg)
	if cfg.SampleRate < 0.0001 || cfg.SampleRate > 1.0 {
		log.Fatalf("invalid sample rate %f provided. allowed value: 0.0001 <= sample-rate <= 1.0", cfg.SampleRate)
	}

	cfg.SampleRate = math.Round(cfg.SampleRate*10000) / 10000
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// set logger
	zapcfg := zap.NewProductionConfig()
	zapcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zapcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapcfg.Encoding = "console"
	zapcfg.Level = zap.NewAtomicLevelAt(*logLevel)
	logger, err := zapcfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	grpclog.SetLoggerV2(zapgrpc.NewLogger(logger))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()

	if err := tracegen.SendDistributedTrace(ctx, &cfg, logger.Sugar()); err != nil {
		logger.Fatal("error sending distributed tracing data", zap.Error(err))
	}
}

// configureEnv parses or sets env configs to work with both Elastic GO Agent and OTLP library
func configureEnv(cfg *tracegen.Config) error {
	// if API Key is not supplied by flags, get it from env
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("ELASTIC_APM_API_KEY")
	}

	if cfg.APMServerURL == "" {
		cfg.APMServerURL = os.Getenv("ELASTIC_APM_SERVER_URL")
	}

	if cfg.APIKey == "" || cfg.APMServerURL == "" {
		return errors.New("API Key and APM Server URL must be configured")
	}
	// to supply these to GO Agent
	os.Setenv("ELASTIC_APM_API_KEY", cfg.APIKey)
	os.Setenv("ELASTIC_APM_SERVER_URL", cfg.APMServerURL)

	return nil
}
