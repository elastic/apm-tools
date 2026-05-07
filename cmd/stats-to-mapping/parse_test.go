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
	"reflect"
	"testing"
)

// TestParseEntryFields_FoldedScalarSibling guards against a regression
// where a folded scalar's continuation line (deeper than the entry's
// mapping-pair indent) caused the parser to break out of the
// surrounding entry, dropping every following sibling. The shape is
// taken straight from elastic/integrations beat-stats-fields.yml.
func TestParseEntryFields_FoldedScalarSibling(t *testing.T) {
	entry := []byte(`    - name: apm_server
      type: group
      description: >
        APM Server specific metrics
      fields:
        - name: sampling
          type: group
          fields:
            - name: transactions_dropped
              type: long
              metric_type: counter
              description: >
                Number of transactions dropped
        - name: server
          type: group
          fields:
            - name: request
              type: group
              fields:
                - name: count
                  type: long
                  metric_type: counter
                  description: >
                    Number of requests received
`)
	got := parseEntryFields(entry, 4)
	want := []item{
		{Name: "sampling", Type: "group", Fields: []item{
			{Name: "transactions_dropped", Type: "long"},
		}},
		{Name: "server", Type: "group", Fields: []item{
			{Name: "request", Type: "group", Fields: []item{
				{Name: "count", Type: "long"},
			}},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\n got: %+v\nwant: %+v", got, want)
	}
}
