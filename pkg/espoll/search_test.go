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
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8"
)

const (
	defaultBody = `{"fields":["*"]}`
	queryBody   = `{"query":{"bool":{}},"fields":["*"]}`
)

func TestDefaultBodyRepeatedOnRetryAfterPoll(t *testing.T) {
	es := newFakeES(t, indexedAfter(1), failRequest(2, status503))
	search(t, es.NewSearchRequest("traces-apm*"))
	assertBodies(t, es.received, 3, defaultBody)
}

func TestDefaultBodyRepeatedOnDroppedConnectionAfterPoll(t *testing.T) {
	es := newFakeES(t, indexedAfter(1), failRequest(2, dropConnection))
	search(t, es.NewSearchRequest("traces-apm*"))
	assertBodies(t, es.received, 3, defaultBody)
}

func TestDefaultBodyRepeatedOnRetryBeforeFirstPoll(t *testing.T) {
	es := newFakeES(t, failRequest(1, status503))
	search(t, es.NewSearchRequest("traces-apm*"))
	assertBodies(t, es.received, 2, defaultBody)
}

func TestDefaultBodyRepeatedAcrossPolls(t *testing.T) {
	es := newFakeES(t, indexedAfter(3))
	search(t, es.NewSearchRequest("traces-apm*"))
	assertBodies(t, es.received, 4, defaultBody)
}

func TestQueryBodyRepeatedOnRetryAfterPoll(t *testing.T) {
	es := newFakeES(t, indexedAfter(1), failRequest(2, status503))
	search(t, es.NewSearchRequest("traces-apm*").WithQuery(BoolQuery{}))
	assertBodies(t, es.received, 3, queryBody)
}

func TestQueryBodyRepeatedOnDroppedConnectionAfterPoll(t *testing.T) {
	es := newFakeES(t, indexedAfter(1), failRequest(2, dropConnection))
	search(t, es.NewSearchRequest("traces-apm*").WithQuery(BoolQuery{}))
	assertBodies(t, es.received, 3, queryBody)
}

// search polls req until it returns at least one hit.
func search(t *testing.T, req *SearchRequest) {
	t.Helper()
	var result SearchResult
	_, err := req.Do(context.Background(), &result,
		WithInterval(time.Millisecond),
		WithTimeout(5*time.Second),
		WithCondition(result.Hits.MinHitsCondition(1)),
	)
	require.NoError(t, err)
	require.Len(t, result.Hits.Hits, 1)
}

func assertBodies(t *testing.T, got []string, wantCount int, wantBody string) {
	t.Helper()
	require.Len(t, got, wantCount)
	for i, body := range got {
		assert.JSONEq(t, wantBody, body, "request %d", i+1)
	}
}

type fakeES struct {
	*Client
	indexedAfter int
	failAt       int
	failWith     failure
	received     []string
}

type failure func() (*http.Response, error)

func indexedAfter(requests int) func(*fakeES) {
	return func(es *fakeES) { es.indexedAfter = requests }
}

func failRequest(n int, with failure) func(*fakeES) {
	return func(es *fakeES) { es.failAt, es.failWith = n, with }
}

// status503 is retried by elastic-transport (default RetryOnStatus).
func status503() (*http.Response, error) {
	return respond(http.StatusServiceUnavailable, `{"error":{"type":"no_shard_available_action_exception"}}`), nil
}

// dropConnection fails without a response, which elastic-transport also
// retries (default RetryOnError).
func dropConnection() (*http.Response, error) {
	return nil, errors.New("connection reset by peer")
}

func newFakeES(t *testing.T, opts ...func(*fakeES)) *fakeES {
	t.Helper()
	es := &fakeES{}
	for _, opt := range opts {
		opt(es)
	}
	client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: es})
	require.NoError(t, err)
	es.Client = WrapClient(client)
	return es
}

func (es *fakeES) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	es.received = append(es.received, string(body))
	n := len(es.received)

	if n == es.failAt {
		return es.failWith()
	}
	if n <= es.indexedAfter {
		return respond(http.StatusOK, `{"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`), nil
	}
	return respond(http.StatusOK, `{"hits":{"total":{"value":1,"relation":"eq"},"hits":[`+
		`{"_index":"traces-apm-default","_id":"1","_source":{"a":1},"fields":{"a":[1]}}]}}`), nil
}

func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"X-Elastic-Product": {"Elasticsearch"}, "Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
