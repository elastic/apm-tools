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
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) sendEventsCommand(c *cli.Context) error {
	creds, err := cmd.getCredentials(c)
	if err != nil {
		return err
	}

	var body io.Reader
	filename := c.String("file")
	if filename == "" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			log.Fatalf("failed to stat stdin: %s", err.Error())
		}
		if stat.Size() == 0 {
			log.Fatal("empty -file flag and stdin, please set one.")
		}
		body = io.NopCloser(os.Stdin)
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("error opening file: %w", err)
		}
		defer f.Close()
		body = f
	}

	urlPath := "/intake/v2/events"
	if c.Bool("rumv2") {
		urlPath = "/intake/v2/rum/events"
	}
	req, err := http.NewRequest(
		http.MethodPost,
		cmd.cfg.APMServerURL+urlPath+"?verbose",
		body,
	)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")

	switch {
	case creds.SecretToken != "":
		req.Header.Set("Authorization", "Bearer "+creds.SecretToken)
	case creds.APIKey != "":
		req.Header.Set("Authorization", "ApiKey "+creds.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error performing HTTP request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stderr, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("error sending events; server responded with %q", resp.Status)
	}
	return nil
}

// NewSendEventCmd returns pointer to a Command that sends events to APM Server
func NewSendEventCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "send-events",
		Usage:  "send events stored in ND-JSON format",
		Action: commands.sendEventsCommand,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Required: true,
				Usage:    "File containing the payload to send, in ND-JSON format. Payload must be provided via this flag or stdin.",
			},
			&cli.BoolFlag{
				Name:  "rumv2",
				Usage: "Send events to /intake/v2/rum/events",
			},
		},
	}
}
