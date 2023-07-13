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

// Package tracegen contains functions that generate a trace including transaction,
// span, error, and logs using elastic APM Go agent and opentelemtry-go
package tracegen

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.elastic.co/apm/v2"
)

// IndexIntakeV2Trace generate a trace including a transaction, a span and an error
func IndexIntakeV2Trace(ctx context.Context, cfg Config, tracer *apm.Tracer) error {
	if cfg.SampleRate < 0.0001 || cfg.SampleRate > 1.0 {
		return errors.New("invalid sample rate provided. allowed value: 0.0001 <= sample-rate <= 1.0")
	}
	cfg.SampleRate = math.Round(cfg.SampleRate*10000) / 10000

	// set sample rate
	ts := apm.NewTraceState(apm.TraceStateEntry{
		Key: "es", Value: fmt.Sprintf("s:%.4g", cfg.SampleRate),
	})

	traceContext := apm.TraceContext{
		Trace:   cfg.TraceID,
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
	tracer.Flush(ctx.Done())

	return nil
}
