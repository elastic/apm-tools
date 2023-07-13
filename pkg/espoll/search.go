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

package espoll

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"
)

// SearchIndexMinDocs searches index with query, returning the results.
//
// If the search returns fewer than min results within 10 seconds
// (by default), SearchIndexMinDocs will return an error.
func (es *Client) SearchIndexMinDocs(
	ctx context.Context,
	min int, index string,
	query json.Marshaler,
	opts ...RequestOption,
) (SearchResult, error) {
	var result SearchResult
	req := es.NewSearchRequest(index)
	req.ExpandWildcards = "open,hidden"
	if min > 10 {
		// Size defaults to 10. If the caller expects more than 10,
		// return it in the search so we don't have to search again.
		req = req.WithSize(min)
	}
	if query != nil {
		req = req.WithQuery(query)
	}
	opts = append(opts, WithCondition(AllCondition(
		result.Hits.MinHitsCondition(min),
		result.Hits.TotalHitsCondition(req),
	)))

	// Refresh the indices before issuing the search request.
	refreshReq := esapi.IndicesRefreshRequest{
		Index:           strings.Split(",", index),
		ExpandWildcards: "all",
	}
	rsp, err := refreshReq.Do(ctx, es.Transport)
	if err != nil {
		return result, fmt.Errorf("failed refreshing indices: %s: %w", index, err)
	}

	rsp.Body.Close()

	if _, err := req.Do(ctx, &result, opts...); err != nil {
		return result, fmt.Errorf("failed issuing request: %w", err)
	}
	return result, nil
}

// NewSearchRequest returns a search request using the wrapped Elasticsearch
// client.
func (es *Client) NewSearchRequest(index string) *SearchRequest {
	req := &SearchRequest{es: es}
	req.Index = strings.Split(index, ",")
	req.Body = strings.NewReader(`{"fields": ["*"]}`)
	return req
}

// SearchRequest wraps an esapi.SearchRequest with a Client.
type SearchRequest struct {
	esapi.SearchRequest
	es *Client
}

func (r *SearchRequest) WithQuery(q any) *SearchRequest {
	var body struct {
		Query  any      `json:"query"`
		Fields []string `json:"fields"`
	}
	body.Query = q
	body.Fields = []string{"*"}
	r.Body = esutil.NewJSONReader(&body)
	return r
}

func (r *SearchRequest) WithSort(fieldDirection ...string) *SearchRequest {
	r.Sort = fieldDirection
	return r
}

func (r *SearchRequest) WithSize(size int) *SearchRequest {
	r.Size = &size
	return r
}

func (r *SearchRequest) Do(ctx context.Context, out *SearchResult, opts ...RequestOption) (*esapi.Response, error) {
	return r.es.Do(ctx, &r.SearchRequest, out, opts...)
}

type SearchResult struct {
	Hits         SearchHits                 `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations"`
}

type SearchHits struct {
	Total SearchHitsTotal `json:"total"`
	Hits  []SearchHit     `json:"hits"`
}

type SearchHitsTotal struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"` // "eq" or "gte"
}

// NonEmptyCondition returns a ConditionFunc which will return true if h.Hits is non-empty.
func (h *SearchHits) NonEmptyCondition() ConditionFunc {
	return h.MinHitsCondition(1)
}

// MinHitsCondition returns a ConditionFunc which will return true if the number of h.Hits
// is at least min.
func (h *SearchHits) MinHitsCondition(min int) ConditionFunc {
	return func(*esapi.Response) bool { return len(h.Hits) >= min }
}

// TotalHitsCondition returns a ConditionFunc which will return true if the number of h.Hits
// is at least h.Total.Value. If the condition returns false, it will update req.Size to
// accommodate the number of hits in the following search.
func (h *SearchHits) TotalHitsCondition(req *SearchRequest) ConditionFunc {
	return func(*esapi.Response) bool {
		if len(h.Hits) >= h.Total.Value {
			return true
		}
		size := h.Total.Value
		req.Size = &size
		return false
	}
}

type SearchHit struct {
	Index     string
	ID        string
	Score     float64
	Fields    map[string][]any
	Source    map[string]any
	RawSource json.RawMessage
	RawFields json.RawMessage
}

func (h *SearchHit) UnmarshalJSON(data []byte) error {
	var searchHit struct {
		Index  string          `json:"_index"`
		ID     string          `json:"_id"`
		Score  float64         `json:"_score"`
		Source json.RawMessage `json:"_source"`
		Fields json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(data, &searchHit); err != nil {
		return err
	}
	h.Index = searchHit.Index
	h.ID = searchHit.ID
	h.Score = searchHit.Score
	h.RawSource = searchHit.Source
	h.RawFields = searchHit.Fields
	h.Source = make(map[string]any)
	h.Fields = make(map[string][]interface{})
	if err := json.Unmarshal(h.RawSource, &h.Source); err != nil {
		return fmt.Errorf("error unmarshaling _source: %w", err)
	}
	if err := json.Unmarshal(h.RawFields, &h.Fields); err != nil {
		return fmt.Errorf("error unmarshaling fields: %w", err)
	}
	return nil
}

func (h *SearchHit) UnmarshalSource(out any) error {
	return json.Unmarshal(h.RawSource, out)
}
