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
	"math/rand"
	"os"
	"os/signal"

	"go.elastic.co/apm/v2"

	"github.com/elastic/apm-tools/pkg/tracegen"
)

func main() {
	var cfg tracegen.Config

	flag.Float64Var(&cfg.SampleRate, "sample-rate", 1.0, "set the sample rate. allowed value: min: 0.0001, max: 1.000")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()
	if err := Main(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}

func Main(ctx context.Context, cfg tracegen.Config) error {
	uniqueName := suffixString("trace")
	serviceName := "service-" + uniqueName
	tracer, err := apm.NewTracer(serviceName, "0.0.1")
	if err != nil {
		return errors.New("failed to instantiate tracer")
	}

	cfg.TraceID = tracegen.NewRandomTraceID()
	err = tracegen.IndexIntakeV2Trace(ctx, cfg, tracer)

	// TODO: call otlp trace gen with given traceID
	return err
}

func suffixString(s string) string {
	const letter = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return fmt.Sprintf("%s-%s", s, string(b))
}
