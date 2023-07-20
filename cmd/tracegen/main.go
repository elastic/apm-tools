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
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"

	"go.elastic.co/apm"
	"go.uber.org/zap"

	"github.com/elastic/apm-tools/pkg/tracegen"
)

func main() {
	serverURL := flag.String("server", "", "set APM Server URL (env value ELASTIC_APM_SERVER_URL)")
	apiKey := flag.String("api-key", "", "set APM API key for auth (env value ELASTIC_APM_API_KEY)")
	sr := flag.Float64("sample-rate", 1.0, "set the sample rate. allowed value: min: 0.0001, max: 1.000")
	insecure := flag.Bool("insecure", false, "sets agents to skip the server's TLS certificate verification")
	protocol := flag.String("otlp-protocol", "grpc", "set OTLP transport protocol to one of: grpc (default), http/protobuf")
	flag.Parse()

	apmTracer, err := apm.NewTracer(getUniqueServiceName("service", "intake"), "0.0.1")
	if err != nil {
		log.Fatal("failed to instantiate apm tracer")
	}

	cfg := tracegen.NewConfig(
		tracegen.WithAPMServerURL(*serverURL),
		tracegen.WithAPIKey(*apiKey),
		tracegen.WithSampleRate(*sr),
		tracegen.WithInsecureConn(*insecure),
		tracegen.WithElasticAPMTracer(apmTracer),
		tracegen.WithOTLPProtocol(*protocol),
		tracegen.WithOTLPServiceName(getUniqueServiceName("service", "otlp")),
	)

	err = cfg.Validate()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()

	if err := tracegen.SendDistributedTrace(ctx, cfg); err != nil {
		log.Fatal("error sending distributed tracing data", zap.Error(err))
	}
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
