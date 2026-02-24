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

package approvaltest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeSourceAndFields(t *testing.T) {
	cases := []struct {
		name   string
		source map[string]any
		fields map[string][]any
		want   map[string][]any
	}{
		{
			name: "source_only_field_is_included",
			source: map[string]any{
				"error": map[string]any{
					"log": map[string]any{
						"stacktrace": "stack",
					},
				},
			},
			fields: map[string][]any{
				"service.name": {"svc"},
			},
			want: map[string][]any{
				"error.log.stacktrace": {"stack"},
				"service.name":         {"svc"},
			},
		},
		{
			name: "fields_only_field_is_included",
			source: map[string]any{
				"service": map[string]any{
					"name": "svc",
				},
			},
			fields: map[string][]any{
				"span.id": {"abc123"},
			},
			want: map[string][]any{
				"service.name": {"svc"},
				"span.id":      {"abc123"},
			},
		},
		{
			name: "overlapping_key_prefers_fields",
			source: map[string]any{
				"service": map[string]any{
					"name": "from-source",
				},
			},
			fields: map[string][]any{
				"service.name": {"from-fields"},
			},
			want: map[string][]any{
				"service.name": {"from-fields"},
			},
		},
		{
			name: "nested_source_array_is_flattened",
			source: map[string]any{
				"error": map[string]any{
					"log": map[string]any{
						"stacktrace": []any{"line-1", "line-2"},
					},
				},
			},
			fields: map[string][]any{},
			want: map[string][]any{
				"error.log.stacktrace": {"line-1", "line-2"},
			},
		},
		{
			name: "empty_source_array_is_preserved",
			source: map[string]any{
				"error": map[string]any{
					"labels": []any{},
				},
			},
			fields: map[string][]any{},
			want: map[string][]any{
				"error.labels": []any{},
			},
		},
		{
			name: "source_descendants_are_skipped_when_fields_has_parent_object",
			source: map[string]any{
				"transaction": map[string]any{
					"duration": map[string]any{
						"histogram": map[string]any{
							"counts": []any{float64(1)},
							"values": []any{float64(0)},
						},
					},
				},
			},
			fields: map[string][]any{
				"transaction.duration.histogram": {
					map[string]any{
						"counts": []any{float64(1)},
						"values": []any{float64(0)},
					},
				},
			},
			want: map[string][]any{
				"transaction.duration.histogram": {
					map[string]any{
						"counts": []any{float64(1)},
						"values": []any{float64(0)},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRaw, err := mergeFieldsAndSource(tc.source, tc.fields)
			require.NoError(t, err)

			var got map[string][]any
			require.NoError(t, json.Unmarshal(gotRaw, &got))
			require.Equal(t, tc.want, got)
		})
	}
}
