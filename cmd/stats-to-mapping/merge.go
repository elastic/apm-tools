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

// Merge semantics for the regen output.
//
// The tool emits new field definitions for the apm-server and output
// subtrees of each upstream mapping file. apm-server's history of
// removed and renamed metrics (e.g. the Jaeger removal in #14791,
// agentcfg.elasticsearch counter renames) means the upstream files
// accumulate entries the current /stats no longer reports. Replacing
// the subtree wholesale would silently delete those entries; downstream
// dashboards and queries that still reference the old field names would
// break the next time the regen output is committed.
//
// To avoid that, mergeProperties (JSON) and mergeFields (YAML) preserve
// every upstream entry that has no counterpart in the new tree, and
// recursively merge group entries that exist on both sides. Conflicts
// (one side leaf, other side group; or scalar type mismatches) prefer
// the new entry, on the assumption that /stats is the canonical source
// for the current schema.

// mergeProperties merges an Elasticsearch index template's properties
// map (existing) with the regen tool's freshly produced properties map
// (new_). The shape on both sides is {fieldName: entry}, where entry is
// either {"type": "<scalar>"}, {"type": "alias", "path": ...}, or
// {"properties": {...}} for nested groups.
//
// Returns existing if new_ is empty; returns new_ if existing is empty;
// otherwise produces a fresh map with every key from existing plus
// every key from new_, recursing into the "properties" map of group
// entries when both sides agree the entry is a group.
func mergeProperties(existing, new_ map[string]any) map[string]any {
	if len(existing) == 0 {
		return new_
	}
	if len(new_) == 0 {
		return existing
	}
	out := make(map[string]any, len(existing)+len(new_))
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range new_ {
		ev, has := out[k]
		if !has {
			out[k] = v
			continue
		}
		em, eOK := ev.(map[string]any)
		nm, nOK := v.(map[string]any)
		if !eOK || !nOK {
			out[k] = v
			continue
		}
		eProps, eIsGroup := em["properties"].(map[string]any)
		nProps, nIsGroup := nm["properties"].(map[string]any)
		if eIsGroup && nIsGroup {
			out[k] = map[string]any{"properties": mergeProperties(eProps, nProps)}
			continue
		}
		// One side is a leaf, or shapes disagree (e.g. group → leaf
		// because a metric was demoted from a sub-tree to a single
		// value). Prefer the new entry.
		out[k] = v
	}
	return out
}

// mergeFields merges a YAML field tree (existing) with the regen tool's
// freshly produced field tree (new_). Both sides are []item with the
// shape used elsewhere in this package: groups have Type=="group" and
// non-empty Fields; aliases have Type=="alias" and Path; leaves have a
// scalar Type.
//
// Order is preserved: every upstream item retains its position; new
// items not present upstream are appended in their input order.
// Conflicts prefer the new item, with one exception — when both sides
// are groups, their children are recursively merged.
func mergeFields(existing, new_ []item) []item {
	if len(existing) == 0 {
		return new_
	}
	if len(new_) == 0 {
		return existing
	}
	out := make([]item, len(existing))
	copy(out, existing)
	indexByName := make(map[string]int, len(out))
	for i, it := range out {
		indexByName[it.Name] = i
	}
	for _, n := range new_ {
		i, has := indexByName[n.Name]
		if !has {
			indexByName[n.Name] = len(out)
			out = append(out, n)
			continue
		}
		if out[i].Type == "group" && n.Type == "group" {
			out[i].Fields = mergeFields(out[i].Fields, n.Fields)
			continue
		}
		out[i] = n
	}
	return out
}
