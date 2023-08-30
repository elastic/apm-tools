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
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/transport"
)

// SendIntakeV2Trace generate a trace including a transaction, a span and an error
func SendIntakeV2Trace(ctx context.Context, cfg Config) (apm.TraceContext, EventStats, error) {
	if err := cfg.validate(); err != nil {
		return apm.TraceContext{}, EventStats{}, err
	}

	tracer, err := newTracer(cfg)
	if err != nil {
		return apm.TraceContext{}, EventStats{}, fmt.Errorf("failed to create tracer: %w", err)
	}
	defer tracer.Close()

	// set sample rate
	ts := apm.NewTraceState(apm.TraceStateEntry{
		Key: "es", Value: fmt.Sprintf("s:%.4g", cfg.sampleRate),
	})

	traceContext := apm.TraceContext{
		Trace:   cfg.traceID,
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
	tracerStats := tracer.Stats()
	stats := EventStats{
		ExceptionsSent: int(tracerStats.ErrorsSent),
		SpansSent:      int(tracerStats.SpansSent + tracerStats.TransactionsSent),
	}

	return tx.TraceContext(), stats, nil
}

func newTracer(cfg Config) (*apm.Tracer, error) {
	apmServerURL, err := url.Parse(cfg.apmServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	var apmServerTLSConfig *tls.Config
	if cfg.insecure {
		apmServerTLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	apmTransport, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{
		ServerURLs:      []*url.URL{apmServerURL},
		APIKey:          cfg.apiKey,
		UserAgent:       "apm-tool",
		TLSClientConfig: apmServerTLSConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create APM transport: %w", err)
	}
	return apm.NewTracerOptions(apm.TracerOptions{
		ServiceName:    cfg.apmServiceName,
		ServiceVersion: "0.0.1",
		Transport:      apmTransport,
	})
}
