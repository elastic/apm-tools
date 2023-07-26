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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/elastic/apm-tools/pkg/espoll"
	"github.com/elastic/go-elasticsearch/v8"
)

var maxElasticsearchBackoff = 10 * time.Second

type config struct {
	query      string
	esURL      string
	esUsername string
	esPassword string

	target  string
	timeout time.Duration
	hits    uint
}

func (cmd *Commands) pollDocs(c *cli.Context) error {
	cfg := config{
		query:      c.String("query"),
		esURL:      cmd.cfg.ElasticsearchURL,
		esUsername: cmd.cfg.Username,
		esPassword: cmd.cfg.Password,

		target:  c.String("target"),
		timeout: c.Duration("timeout"),
		hits:    c.Uint("min-hits"),
	}
	query := c.String("query")
	if query == "" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			log.Fatalf("failed to stat stdin: %s", err.Error())
		}
		if stat.Size() == 0 {
			log.Fatal("empty -query flag and stdin, please set one.")
		}
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("failed to read stdin: %s", err.Error())
		}
		query = string(strings.Trim(string(b), "\n"))
	}

	log.Println("query:", query)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	if err := Main(ctx, cfg); err != nil {
		log.Fatalf("ERROR: %s", err.Error())
	}

	return nil
}

// NewESPollCmd returns pointer to Command that queries documents from Elasticsearch
func NewESPollCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "espoll",
		Usage:  "poll documents from Elasticsearch",
		Action: commands.pollDocs,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "query",
				Usage: "The Elasticsearch query in Query DSL. Must be set via this flag or stdin.",
			},
			&cli.StringFlag{
				Name:  "target",
				Value: "traces-*,logs-*,metrics-*",
				Usage: "Comma-separated list of data streams, indices, and aliases to search (Supports wildcards (*)).",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 30 * time.Second,
				Usage: "Elasticsearch request timeout",
			},
			&cli.UintFlag{
				Name:  "min-hits",
				Value: 1,
				Usage: "When specified and > 10, this should cause the size parameter to be set.",
			},
		},
	}
}

func Main(ctx context.Context, cfg config) error {
	if cfg.query == "" {
		return errors.New("query cannot be empty")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Username:   cfg.esUsername,
		Password:   cfg.esPassword,
		Addresses:  strings.Split(cfg.esURL, ","),
		Transport:  transport,
		MaxRetries: 5,
		RetryBackoff: func(attempt int) time.Duration {
			backoff := (500 * time.Millisecond) * (1 << (attempt - 1))
			if backoff > maxElasticsearchBackoff {
				backoff = maxElasticsearchBackoff
			}
			return backoff
		},
	})
	if err != nil {
		return err
	}
	esClient := espoll.WrapClient(client)
	result, err := esClient.SearchIndexMinDocs(ctx,
		int(cfg.hits), cfg.target, stringMarshaler(cfg.query),
		espoll.WithTimeout(cfg.timeout),
	)
	if err != nil {
		return fmt.Errorf("search request returned error: %w", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return fmt.Errorf("failed to encode search result: %w", err)
	}
	return nil
}

type stringMarshaler string

func (s stringMarshaler) MarshalJSON() ([]byte, error) { return []byte(s), nil }
