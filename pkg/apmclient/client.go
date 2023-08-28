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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tidwall/gjson"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/security/createapikey"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
)

type Client struct {
	es *elasticsearch.TypedClient
}

// New returns a new Client for querying APM data.
func New(cfg Config) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: cfg.TLSSkipVerify}

	es, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{cfg.ElasticsearchURL},
		Username:  cfg.Username,
		APIKey:    cfg.APIKey,
		Password:  cfg.Password,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch client: %w", err)
	}
	return &Client{
		es: es,
	}, nil
}

// GetElasticCloudAPMInput returns the APM configuration as defined
// in the "elastic-cloud-apm" integration policy,
func (c *Client) GetElasticCloudAPMInput(ctx context.Context) (gjson.Result, error) {
	size := 1
	resp, err := c.es.Search().Index(".fleet-policies").Request(&search.Request{
		Size: &size,
		Sort: []types.SortCombinations{types.SortOptions{
			SortOptions: map[string]types.FieldSort{
				"revision_idx": {
					Order: &sortorder.Desc,
				},
			},
		}},
		Query: &types.Query{
			Term: map[string]types.TermQuery{
				"policy_id": {
					Value: "policy-elastic-agent-on-cloud",
				},
			},
		},
	}).Do(ctx)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("error searching .fleet-policies: %w", err)
	}
	if n := len(resp.Hits.Hits); n != 1 {
		return gjson.Result{}, fmt.Errorf("expected 1 policy, got %d", n)
	}
	result := gjson.GetBytes(resp.Hits.Hits[0].Source_, `data.inputs.#(id=="elastic-cloud-apm")`)
	if !result.Exists() {
		return gjson.Result{}, fmt.Errorf("input %q missing", "elastic-cloud-apm")
	}
	return result, nil
}

// CreateAgentAPIKey creates an agent API Key, and returns it in the
// base64-encoded form that agents should provide.
//
// If expiration is less than or equal to zero, then the API Key never expires.
func (c *Client) CreateAgentAPIKey(ctx context.Context, expiration time.Duration) (string, error) {
	name := "apm-agent"
	var maybeExpiration types.Duration
	if expiration > 0 {
		maybeExpiration = expiration
	}
	resp, err := c.es.Security.CreateApiKey().Request(&createapikey.Request{
		Name:       &name,
		Expiration: maybeExpiration,
		RoleDescriptors: map[string]types.RoleDescriptor{
			"apm": {
				Applications: []types.ApplicationPrivileges{{
					Application: "apm",
					Resources:   []string{"*"},
					Privileges:  []string{"event:write", "config_agent:read"},
				}},
			},
		},
		Metadata: map[string]json.RawMessage{
			"application": []byte(`"apm"`),
			"creator":     []byte(`"apmclient"`),
		},
	}).Do(ctx)
	if err != nil {
		return "", fmt.Errorf("error creating agent API Key: %w", err)
	}
	return resp.Encoded, nil
}

// ServiceSummary returns ServiceSummary objects by aggregating `service_summary` metric sets.
func (c *Client) ServiceSummary(ctx context.Context, options ...Option) ([]ServiceSummary, error) {
	// TODO options
	req := &search.Request{
		Aggregations: map[string]types.Aggregations{
			"services": {
				MultiTerms: &types.MultiTermsAggregation{
					Terms: []types.MultiTermLookup{{
						Field: "service.name",
					}, {
						Field:   "service.environment",
						Missing: "",
					}, {
						Field: "service.language.name",
					}, {
						Field: "agent.name",
					}},
				},
			},
		},
	}
	// TODO select appropriate resolution according to the time filter.
	resp, err := c.es.Search().
		Index("metrics-apm.service_summary.1m-*").
		Size(0).Request(req).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error search service_summmary metrics")
	}

	servicesAggregation := resp.Aggregations["services"].(*types.MultiTermsAggregate)
	buckets := servicesAggregation.Buckets.([]types.MultiTermsBucket)
	out := make([]ServiceSummary, len(buckets))
	for i, bucket := range buckets {
		out[i] = ServiceSummary{
			Name:        bucket.Key[0].(string),
			Environment: bucket.Key[1].(string),
			Language:    bucket.Key[2].(string),
			Agent:       bucket.Key[3].(string),
		}
	}
	return out, nil
}
