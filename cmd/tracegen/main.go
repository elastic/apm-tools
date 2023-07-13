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
	"math/rand"
	"os"
	"os/signal"

	apmhttp "go.elastic.co/apm/module/apmhttp/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapgrpc"
	"google.golang.org/grpc/grpclog"

	"github.com/elastic/apm-tools/pkg/tracegen"
)

func main() {
	var cfg tracegen.Config

	flag.Float64Var(&cfg.SampleRate, "sample-rate", 1.0, "set the sample rate. allowed value: min: 0.0001, max: 1.000")
	flag.StringVar(&cfg.OTLPProtocol, "protocol", "grpc", "set transport protocol to one of: grpc (default), http/protobuf")
	flag.BoolVar(&cfg.Insecure, "insecure", false, "skip the server's TLS certificate verification")
	logLevel := zap.LevelFlag(
		"loglevel", zapcore.InfoLevel,
		"set log level to one of: DEBUG, INFO (default), WARN, ERROR, DPANIC, PANIC, FATAL",
	)

	flag.Parse()
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
	if err := Main(ctx, cfg, logger.Sugar()); err != nil {
		logger.Fatal("error sending data", zap.Error(err))
	}
}

func Main(ctx context.Context, cfg tracegen.Config, otlogger *zap.SugaredLogger) error {
	// set up intake tracegen
	tracer, err := apm.NewTracer(getUniqueServiceName("service", "intake"), "0.0.1")
	if err != nil {
		return errors.New("failed to instantiate apm tracer")
	}

	traceCtx, err := tracegen.IndexIntakeV2Trace(ctx, cfg, tracer)
	if err != nil {
		return err
	}

	traceparent := apmhttp.FormatTraceparentHeader(traceCtx)
	tracestate := traceCtx.State.String()
	ctx = tracegen.SetTracePropagator(ctx, traceparent, tracestate)
	return tracegen.IndexOTLPTrace(ctx, cfg, otlogger, getUniqueServiceName("service", "otlp"))
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
