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
	"encoding/json"
	"fmt"
	"os"
	"time"
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
