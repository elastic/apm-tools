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
	"fmt"
	"math/rand"
	"os"
	"os/signal"

	"github.com/urfave/cli/v3"

	"github.com/elastic/apm-tools/pkg/tracegen"
)

func (cmd *Commands) sendTrace(ctx context.Context, c *cli.Command) error {
	creds, err := cmd.getCredentials(ctx, c)
	if err != nil {
		return err
	}

	cfg := tracegen.NewConfig(
		tracegen.WithAPMServerURL(cmd.cfg.APMServerURL),
		tracegen.WithAPIKey(creds.APIKey),
		tracegen.WithSampleRate(c.Float("sample-rate")),
		tracegen.WithInsecureConn(cmd.cfg.TLSSkipVerify),
		tracegen.WithOTLPProtocol(c.String("otlp-protocol")),
		tracegen.WithOTLPServiceName(newUniqueServiceName("service", "otlp")),
		tracegen.WithElasticAPMServiceName(newUniqueServiceName("service", "intake")),
	)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()

	stats, err := tracegen.SendDistributedTrace(ctx, cfg)
	if err != nil {
		return fmt.Errorf("error sending distributed trace: %w", err)
	}
	fmt.Printf(
		"Sent %d span%s, %d exception%s, and %d log%s\n",
		stats.SpansSent, pluralize(stats.SpansSent),
		stats.ExceptionsSent, pluralize(stats.ExceptionsSent),
		stats.LogsSent, pluralize(stats.LogsSent),
	)

	return nil
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func newUniqueServiceName(prefix string, suffix string) string {
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

// NewTraceGenCmd returns pointer to a Command that generates distributed tracing data using go-agent and otel library
func NewTraceGenCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "generate-trace",
		Usage:  "generate distributed tracing data using go-agent and otel library",
		Action: commands.sendTrace,
		Flags: []cli.Flag{
			&cli.FloatFlag{
				Name:  "sample-rate",
				Usage: "set the sample rate. allowed value: min: 0.0001, max: 1.000",
				Value: 1.0,
			},
			&cli.StringFlag{
				Name:  "otlp-protocol",
				Usage: "set OTLP transport protocol to one of: grpc (default), http/protobuf",
				Value: "grpc",
			},
		},
	}
}
