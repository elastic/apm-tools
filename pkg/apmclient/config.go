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

package apmclient

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	// ElasticsearchURL holds the Elasticsearch URL.
	ElasticsearchURL string

	// Username holds the Elasticsearch username for basic auth.
	Username string

	// Password holds the Elasticsearch password for basic auth.
	Password string

	// APIKey holds an Elasticsearch API Key.
	//
	// This will be set from $ELASTICSEARCH_API_KEY if specified.
	APIKey string

	// APMServerURL holds the APM Server URL.
	//
	// If this is unspecified, it will be derived from
	// ElasticsearchURL if that is an Elastic Cloud URL.
	APMServerURL string

	// KibanaURL holds the Kibana URL.
	//
	// If this is unspecified, it will be derived from
	// ElasticsearchURL if that is an Elastic Cloud URL.
	KibanaURL string

	// TLSSkipVerify determines if TLS certificate
	// verification is skipped or not. Default to false.
	//
	// If not specified the value will be take from
	// TLS_SKIP_VERIFY env var.
	// Any value different from "" is considered true.
	TLSSkipVerify bool
}

// NewConfig returns a Config intialised from environment variables.
func NewConfig() (Config, error) {
	cfg := Config{}
	err := cfg.Finalize()
	return cfg, err
}

// Finalize finalizes cfg by setting unset fields from environment
// variables:
//
//   - ElasticsearchURL is set from $ELASTICSEARCH_URL
//   - Username is set from $ELASTICSEARCH_USERNAME
//   - Password is set from $ELASTICSEARCH_PASSWORD
//   - API Key is set from $ELASTICSEARCH_API_KEY
//   - APMServerURL is set from $ELASTIC_APM_SERVER_URL
//   - KibanaURL is set from $KIBANA_URL
//
// If $ELASTIC_APM_SERVER_URL is unspecified, and ElasticsearchURL
// holds an Elastic Cloud-based URL, then the APM Server URL is
// derived from that. Likewise, the Kibana URL will be set in this
// way if $KIBANA_URL is unspecified.
func (cfg *Config) Finalize() error {
	if cfg.ElasticsearchURL == "" {
		cfg.ElasticsearchURL = os.Getenv("ELASTICSEARCH_URL")
	}
	if cfg.Username == "" {
		cfg.Username = os.Getenv("ELASTICSEARCH_USERNAME")
	}
	if cfg.Password == "" {
		cfg.Password = os.Getenv("ELASTICSEARCH_PASSWORD")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("ELASTICSEARCH_API_KEY")
	}
	if cfg.APMServerURL == "" {
		cfg.APMServerURL = os.Getenv("ELASTIC_APM_SERVER_URL")
	}
	if cfg.KibanaURL == "" {
		cfg.KibanaURL = os.Getenv("KIBANA_URL")
	}
	if env := os.Getenv("TLS_SKIP_VERIFY"); !cfg.TLSSkipVerify && env != "" {
		cfg.TLSSkipVerify = true
	}
	return cfg.InferElasticCloudURLs()
}

// InferElasticCloudURLs attempts to infer a value for APMServerURL
// and KibanaURL (if they are empty), by checking if ElasticsearchURL
// matches an Elastic Cloud URL pattern, and deriving the other URLs
// from that.
func (cfg *Config) InferElasticCloudURLs() error {
	if cfg.ElasticsearchURL == "" {
		return nil
	}
	if cfg.APMServerURL != "" && cfg.KibanaURL != "" {
		return nil
	}

	// If ElasticsearchURL matches https://<alias>.es.<...>
	// then derive the APM Server URL from that by substituting
	// "apm" for "es", and Kibana URL by substituing "kb".
	url, err := url.Parse(cfg.ElasticsearchURL)
	if err != nil {
		return fmt.Errorf("error parsing ElasticsearchURL: %w", err)
	}
	if alias, remainder, ok := strings.Cut(url.Host, "."); ok {
		if component, remainder, ok := strings.Cut(remainder, "."); ok && component == "es" {
			if cfg.APMServerURL == "" {
				url.Host = fmt.Sprintf("%s.apm.%s", alias, remainder)
				cfg.APMServerURL = url.String()
			}
			if cfg.KibanaURL == "" {
				url.Host = fmt.Sprintf("%s.kb.%s", alias, remainder)
				cfg.KibanaURL = url.String()
			}
		}
	}
	return nil
}
