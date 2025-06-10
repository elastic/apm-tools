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

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) envCommand(ctx context.Context, c *cli.Command) error {
	if cmd.cfg.ElasticsearchURL == "" {
		return fmt.Errorf("Elasticsearch URL is not set")
	}
	if cmd.cfg.Username == "" {
		return fmt.Errorf("Elasticsearch username is not set")
	}
	if cmd.cfg.Password == "" {
		return fmt.Errorf("Elasticsearch password is not set")
	}

	creds, err := cmd.getCredentials(ctx, c)
	if err != nil {
		return err
	}

	fmt.Printf("export ELASTIC_APM_SERVER_URL=%q;\n", cmd.cfg.APMServerURL)
	if creds.APIKey != "" {
		fmt.Printf("export ELASTIC_APM_API_KEY=%q;\n", creds.APIKey)
	} else if creds.SecretToken != "" {
		fmt.Printf("export ELASTIC_APM_SECRET_TOKEN=%q;\n", creds.SecretToken)
	}

	fmt.Printf("export OTEL_EXPORTER_OTLP_ENDPOINT=%q;\n", cmd.cfg.APMServerURL)
	if creds.APIKey != "" {
		fmt.Printf("export OTEL_EXPORTER_OTLP_HEADERS=%q;\n", "Authorization=ApiKey "+creds.APIKey)
	} else if creds.SecretToken != "" {
		fmt.Printf("export OTEL_EXPORTER_OTLP_HEADERS=%q;\n", "Authorization=Bearer "+creds.SecretToken)
	}
	return nil
}

// NewPrintEnvCmd returns pointer to a Command that prints environment variables for configuring Elastic APM agent
func NewPrintEnvCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "agent-env",
		Usage:  "print environment variables for configuring Elastic APM agents",
		Action: commands.envCommand,
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:  "api-key-expiration",
				Usage: "specify how long before a created API Key expires. 0 means it never expires.",
			},
		},
	}
}
