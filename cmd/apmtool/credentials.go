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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v3"
)

type credentials struct {
	Expiry      time.Time `json:"expiry,omitempty"`
	APIKey      string    `json:"api_key,omitempty"`
	SecretToken string    `json:"secret_token,omitempty"`
}

// readCachedCredentials returns any cached credentials for the given URL.
// If there are no cached credentials, readCachedCredentials returns an error
// satisfying errors.Is(err, os.ErrNotExist).
func readCachedCredentials(url string) (*credentials, error) {
	data, err := readCache("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("error reading cached credentials: %w", err)
	}
	var m map[string]*credentials
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("error decoding cached credentials: %w", err)
	}
	if c, ok := m[url]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("no credentials cached for %q: %w", url, os.ErrNotExist)
}

// updateCachedCredentials updates credentials for the given URL.
//
// Any expired credentials will be remove from the cache.
func updateCachedCredentials(url string, c *credentials) error {
	if err := updateCache("credentials.json", func(data []byte) ([]byte, error) {
		m := make(map[string]*credentials)
		if data != nil {
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, err
			}
		}
		m[url] = c
		now := time.Now()
		for k, v := range m {
			if !v.Expiry.IsZero() && v.Expiry.Before(now) {
				delete(m, k)
			}
		}
		return json.Marshal(m)
	}); err != nil {
		return fmt.Errorf("error updating cached credentials: %w", err)
	}
	return nil
}

func (cmd *Commands) getCredentials(ctx context.Context, c *cli.Command) (*credentials, error) {
	creds, err := readCachedCredentials(cmd.cfg.APMServerURL)
	if err == nil {
		return creds, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	client, err := cmd.getClient()
	if err != nil {
		return nil, err
	}

	var expiry time.Time
	// First check if there's an Elastic Cloud integration policy,
	// and extract a secret token from that. Otherwise, create an
	// API Key.
	var apiKey, secretToken string
	policy, err := client.GetElasticCloudAPMInput(ctx)
	policyErr := fmt.Errorf("error getting APM cloud input: %w", err)
	if err != nil {
		if c.Bool("verbose") {
			fmt.Fprintln(os.Stderr, policyErr)
		}
	} else {
		secretToken = policy.Get("apm-server.auth.secret_token").String()
	}
	// Create an API Key.
	fmt.Fprintln(os.Stderr, "Creating agent API Key...")
	expiryDuration := c.Duration("api-key-expiration")
	if expiryDuration > 0 {
		expiry = time.Now().Add(expiryDuration)
	}
	apiKey, err = client.CreateAgentAPIKey(ctx, expiryDuration)
	if err != nil {
		apiKeyErr := err
		return nil, fmt.Errorf(
			"failed to obtain agent credentials: %w",
			errors.Join(apiKeyErr, policyErr),
		)
	}
	creds = &credentials{
		Expiry:      expiry,
		APIKey:      apiKey,
		SecretToken: secretToken,
	}
	if err := updateCachedCredentials(cmd.cfg.APMServerURL, creds); err != nil {
		return nil, err
	}
	return creds, nil
}
