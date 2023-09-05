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

package metricgen

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"go.elastic.co/apm/module/apmotel/v2"
	"go.elastic.co/apm/v2"
	"go.opentelemetry.io/otel/sdk/metric"
)

// SendIntakeV2 sends specific metrics to the configured Elastic APM intake V2.
//
// Metrics sent are:
// - apm(float64, value=1.0); gathered from a apm.MetricGatherer
// - apmotel(float64, value=1.0); gathered from a otel MeterProvider through apmotel bridge
// All builtin APM Agent metrics have been disabled.
func SendIntakeV2(ctx context.Context, cfg Config) (EventStats, error) {
	if err := cfg.Validate(); err != nil {
		return EventStats{}, fmt.Errorf("cannot validate IntakeV2 Metrics configuration: %w", err)
	}

	// disable builtin metrics entirely to have predictable metrics value.
	os.Setenv("ELASTIC_APM_DISABLE_METRICS", "system.*, *cpu*, *golang*")
	os.Setenv("ELASTIC_APM_VERIFY_SERVER_CERT", strconv.FormatBool(cfg.verifyServerCert))

	stats := EventStats{}

	tracer, err := apm.NewTracer(cfg.apmServiceName, "0.0.1")
	if err != nil {
		return EventStats{}, fmt.Errorf("cannot setup a tracer: %w", err)
	}

	// setup apmotel bridge to test metrics coming from OTLP
	exporter, err := apmotel.NewGatherer()
	if err != nil {
		return EventStats{}, fmt.Errorf("cannot init gatherer: %w", err)
	}

	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	o := tracer.RegisterMetricsGatherer(exporter)
	defer o()

	d := tracer.RegisterMetricsGatherer(Gatherer{})
	defer d()

	meter := provider.Meter("metricgen")
	counter, _ := meter.Float64Counter("apmotel")
	counter.Add(context.Background(), 1)
	stats.Add(1)

	tracer.SendMetrics(nil)
	stats.Add(1)

	tracer.Flush(nil)

	return stats, nil
}

type Gatherer struct {
}

// GatherMetrics gathers metrics into out.
func (e Gatherer) GatherMetrics(ctx context.Context, out *apm.Metrics) error {
	out.Add("apm", nil, 1.0)
	return nil
}
