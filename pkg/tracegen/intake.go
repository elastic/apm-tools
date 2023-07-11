package tracegen

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.elastic.co/apm/v2"
)

type Config struct {
	SampleRate float64
	TraceID    apm.TraceID
}

// IndexIntakeV2Trace generate a trace including a transaction, a span and an error
func IndexIntakeV2Trace(ctx context.Context, cfg Config, tracer *apm.Tracer) (apm.TraceID, error) {
	// flush before creating a new trace
	tracer.Flush(ctx.Done())

	if cfg.SampleRate < 0.0001 || cfg.SampleRate > 1.0 {
		return cfg.TraceID, errors.New("invalid sample rate provided. allowed value: 0.0001 <= sample-rate <= 1.0")
	}
	cfg.SampleRate = math.Round(cfg.SampleRate*10000) / 10000

	// set sample rate
	ts := apm.NewTraceState(apm.TraceStateEntry{
		Key: "es", Value: fmt.Sprintf("s:%.4g", cfg.SampleRate),
	})

	traceID := cfg.TraceID
	if traceID.Validate() != nil {
		binary.LittleEndian.PutUint64(traceID[:8], rand.Uint64())
		binary.LittleEndian.PutUint64(traceID[8:], rand.Uint64())
	}
	traceContext := apm.TraceContext{
		Trace:   traceID,
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

	return traceID, nil
}
