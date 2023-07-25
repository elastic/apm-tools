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
	"log"
	"math/rand"
	"os"
	"os/signal"

	"go.elastic.co/apm/v2"
	"go.uber.org/zap"

	"github.com/urfave/cli/v3"

	"github.com/elastic/apm-tools/pkg/tracegen"
)

func (cmd *Commands) sendTrace(c *cli.Context) error {
	creds, err := cmd.getCredentials(c)
	if err != nil {
		return err
	}
	apmTracer, err := apm.NewTracer(newUniqueServiceName("service", "intake"), "0.0.1")
	if err != nil {
		log.Fatal("failed to instantiate apm tracer")
	}
	cfg := tracegen.NewConfig(
		tracegen.WithAPMServerURL(cmd.cfg.APMServerURL),
		tracegen.WithAPIKey(creds.APIKey),
		tracegen.WithSampleRate(c.Float64("sample-rate")),
		tracegen.WithInsecureConn(c.Bool("insecure")),
		tracegen.WithElasticAPMTracer(apmTracer),
		tracegen.WithOTLPProtocol(c.String("otlp-protocol")),
		tracegen.WithOTLPServiceName(newUniqueServiceName("service", "otlp")),
	)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()

	if err := tracegen.SendDistributedTrace(ctx, cfg); err != nil {
		log.Fatal("error sending distributed tracing data", zap.Error(err))
	}
	return nil
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

func NewTraceGenCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "tracegen",
		Usage:  "generate distributed tracing data using go-agent and otel library",
		Action: commands.sendTrace,
		Flags: []cli.Flag{
			&cli.Float64Flag{
				Name:  "sample-rate",
				Usage: "set the sample rate. allowed value: min: 0.0001, max: 1.000",
				Value: 1.0,
			},
			&cli.BoolFlag{
				Name:  "insecure",
				Usage: "sets agents to skip the server's TLS certificate verification",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "otlp-protocol",
				Usage: "set OTLP transport protocol to one of: grpc (default), http/protobuf",
				Value: "grpc",
			},
		},
	}
}
