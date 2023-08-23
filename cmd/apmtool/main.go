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
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	commands := &Commands{}
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:       "verbose",
				Usage:      "print debugging messages about progress",
				Aliases:    []string{"v"},
				Persistent: true,
			},
			&cli.StringFlag{
				Name:        "url",
				Usage:       "set the Elasticsearch URL",
				Category:    "Elasticsearch",
				Value:       "",
				Sources:     cli.EnvVars("ELASTICSEARCH_URL"),
				Destination: &commands.cfg.ElasticsearchURL,
				Persistent:  true,
				Action: func(c *cli.Context, v string) error {
					return commands.cfg.InferElasticCloudURLs()
				},
			},
			&cli.StringFlag{
				Name:        "username",
				Usage:       "set the Elasticsearch username",
				Category:    "Elasticsearch",
				Value:       "elastic",
				Sources:     cli.EnvVars("ELASTICSEARCH_USERNAME"),
				Destination: &commands.cfg.Username,
				Persistent:  true,
			},
			&cli.StringFlag{
				Name:        "password",
				Usage:       "set the Elasticsearch password",
				Category:    "Elasticsearch",
				Sources:     cli.EnvVars("ELASTICSEARCH_PASSWORD"),
				Destination: &commands.cfg.Password,
				Persistent:  true,
			},
			&cli.StringFlag{
				Name:        "api-key",
				Usage:       "set the Elasticsearch API Key",
				Category:    "Elasticsearch",
				Sources:     cli.EnvVars("ELASTICSEARCH_API_KEY"),
				Destination: &commands.cfg.APIKey,
				Persistent:  true,
			},
			&cli.StringFlag{
				Name:        "apm-url",
				Usage:       "set the APM Server URL. Will be derived from the Elasticsearch URL for Elastic Cloud.",
				Category:    "APM",
				Value:       "",
				Sources:     cli.EnvVars("ELASTIC_APM_SERVER_URL"),
				Destination: &commands.cfg.APMServerURL,
				Persistent:  true,
			},
			&cli.BoolFlag{
				Name:        "insecure",
				Usage:       "skip TLS certificate verification of Elasticsearch and APM server",
				Value:       false,
				Sources:     cli.EnvVars("TLS_SKIP_VERIFY"),
				Destination: &commands.cfg.TLSSkipVerify,
				Persistent:  true,
			},
		},
		Commands: []*cli.Command{
			NewPrintEnvCmd(commands),
			NewSendEventCmd(commands),
			NewUploadSourcemapCmd(commands),
			NewListServiceCmd(commands),
			NewTraceGenCmd(commands),
			NewESPollCmd(commands),
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
