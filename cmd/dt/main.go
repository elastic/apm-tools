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
	"time"

	"go.elastic.co/apm/v2"
)

type config struct {
	sampleRate float64
}

func main() {
	var cfg config

	flag.Float64Var(&cfg.sampleRate, "sample-rate", 1.0, "set the sample rate. allowed value: min: 0.1, max: 1.0")
	flag.Parse()

	if err := Main(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}
}

func Main(ctx context.Context, cfg config) error {
	if cfg.sampleRate < 0.1 || cfg.sampleRate > 1.0 {
		return errors.New("Invalid sample rate provided. allowed value: 0.1 <= sample-rate <= 1.0")
	}

	_, err := indexIntakeV2Trace(cfg, 0)

	return err
}

func indexIntakeV2Trace(cfg config, idOverride byte) (byte, error) {
	uniqueName := suffixString("trace")
	serviceName := "service-" + uniqueName
	tracer, err := apm.NewTracer(serviceName, "0.0.1")

	if err != nil {
		return 0, errors.New("Failed to instantiate tracer")
	}
	// set sample rate
	ts := apm.NewTraceState(apm.TraceStateEntry{
		Key: "es", Value: fmt.Sprintf("s:%.4g", cfg.sampleRate),
	})
	traceID := byte((rand.Float64() + 0.1) * 999)
	if idOverride != 0 {
		traceID = idOverride
	}
	traceContext := apm.TraceContext{
		Trace:   apm.TraceID{traceID},
		Options: apm.TraceOptions(0).WithRecorded(true),
		State:   ts,
	}

	tx := tracer.StartTransactionOptions("parent-tx", "apmtool", apm.TransactionOptions{
		TraceContext: traceContext,
	})

	span := tx.StartSpanOptions("parent-span", "apmtool", apm.SpanOptions{
		Parent: tx.TraceContext(),
	})

	exit := tx.StartSpanOptions("exit-span", "apmtool", apm.SpanOptions{
		Parent:   span.TraceContext(),
		ExitSpan: true,
	})

	exit.Context.SetServiceTarget(apm.ServiceTargetSpanContext{
		Type: "service_type",
		Name: "service_name",
	})

	exit.Duration = 999 * time.Millisecond
	exit.Outcome = "failure"

	// error
	e := tracer.NewError(errors.New("timeout"))
	e.Culprit = "timeout"
	e.SetSpan(exit)
	e.Send()
	exit.End()

	span.Duration = time.Second
	span.Outcome = "success"
	span.End()
	tx.Duration = 2 * time.Second
	tx.Outcome = "success"
	tx.End()
	tracer.Flush(nil)

	return traceID, nil
}

func suffixString(s string) string {
	const letter = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return fmt.Sprintf("%s-%s", s, string(b))
}
