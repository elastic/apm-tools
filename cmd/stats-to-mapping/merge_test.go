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

func TestMergeProperties(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]any
		new_     map[string]any
		want     map[string]any
	}{{
		name:     "existing only — entry kept",
		existing: map[string]any{"deprecated": map[string]any{"type": "long"}},
		new_:     nil,
		want:     map[string]any{"deprecated": map[string]any{"type": "long"}},
	}, {
		name:     "new only — entry added",
		existing: nil,
		new_:     map[string]any{"fresh": map[string]any{"type": "long"}},
		want:     map[string]any{"fresh": map[string]any{"type": "long"}},
	}, {
		name: "disjoint — both kept",
		existing: map[string]any{
			"deprecated": map[string]any{"type": "long"},
		},
		new_: map[string]any{
			"current": map[string]any{"type": "long"},
		},
		want: map[string]any{
			"deprecated": map[string]any{"type": "long"},
			"current":    map[string]any{"type": "long"},
		},
	}, {
		name: "leaf collision — new wins",
		existing: map[string]any{
			"x": map[string]any{"type": "long"},
		},
		new_: map[string]any{
			"x": map[string]any{"type": "float"},
		},
		want: map[string]any{
			"x": map[string]any{"type": "float"},
		},
	}, {
		name: "groups recurse — children of both kept",
		existing: map[string]any{
			"errors": map[string]any{"properties": map[string]any{
				"deprecated": map[string]any{"type": "long"},
			}},
		},
		new_: map[string]any{
			"errors": map[string]any{"properties": map[string]any{
				"current": map[string]any{"type": "long"},
			}},
		},
		want: map[string]any{
			"errors": map[string]any{"properties": map[string]any{
				"deprecated": map[string]any{"type": "long"},
				"current":    map[string]any{"type": "long"},
			}},
		},
	}, {
		name: "shape mismatch — leaf to group prefers new",
		existing: map[string]any{
			"x": map[string]any{"type": "long"},
		},
		new_: map[string]any{
			"x": map[string]any{"properties": map[string]any{
				"y": map[string]any{"type": "long"},
			}},
		},
		want: map[string]any{
			"x": map[string]any{"properties": map[string]any{
				"y": map[string]any{"type": "long"},
			}},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeProperties(tc.existing, tc.new_)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("\n got: %v\nwant: %v", got, tc.want)
			}
		})
	}
}

func TestMergeFields(t *testing.T) {
	tests := []struct {
		name     string
		existing []item
		new_     []item
		want     []item
	}{{
		name:     "existing only — entry kept",
		existing: []item{{Name: "deprecated", Type: "long"}},
		new_:     nil,
		want:     []item{{Name: "deprecated", Type: "long"}},
	}, {
		name:     "new only — entry added",
		existing: nil,
		new_:     []item{{Name: "fresh", Type: "long"}},
		want:     []item{{Name: "fresh", Type: "long"}},
	}, {
		name: "existing order preserved, new appended",
		existing: []item{
			{Name: "a", Type: "long"},
			{Name: "b", Type: "long"},
		},
		new_: []item{
			{Name: "b", Type: "long"}, // already present
			{Name: "c", Type: "long"}, // new
		},
		want: []item{
			{Name: "a", Type: "long"},
			{Name: "b", Type: "long"},
			{Name: "c", Type: "long"},
		},
	}, {
		name: "leaf collision — new wins",
		existing: []item{
			{Name: "x", Type: "long"},
		},
		new_: []item{
			{Name: "x", Type: "float"},
		},
		want: []item{
			{Name: "x", Type: "float"},
		},
	}, {
		name: "groups recurse — children of both kept",
		existing: []item{
			{Name: "errors", Type: "group", Fields: []item{
				{Name: "deprecated", Type: "long"},
			}},
		},
		new_: []item{
			{Name: "errors", Type: "group", Fields: []item{
				{Name: "current", Type: "long"},
			}},
		},
		want: []item{
			{Name: "errors", Type: "group", Fields: []item{
				{Name: "deprecated", Type: "long"},
				{Name: "current", Type: "long"},
			}},
		},
	}, {
		name: "shape mismatch — leaf to group prefers new",
		existing: []item{
			{Name: "x", Type: "long"},
		},
		new_: []item{
			{Name: "x", Type: "group", Fields: []item{
				{Name: "y", Type: "long"},
			}},
		},
		want: []item{
			{Name: "x", Type: "group", Fields: []item{
				{Name: "y", Type: "long"},
			}},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeFields(tc.existing, tc.new_)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("\n got: %v\nwant: %v", got, tc.want)
			}
		})
	}
}
