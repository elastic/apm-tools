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
	"bytes"
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

type eventType int

const (
	undefinedEventType eventType = iota
	errorEventType
	logEventType
	metricEventType
	spanEventType
	transactionEventType
)

// eventType derives the event type from various fields.
func getEventType(fields json.RawMessage) eventType {
	datastreamType := gjson.GetBytes(fields, `data_stream\.type.0`)
	datastreamDataset := gjson.GetBytes(fields, `data_stream\.dataset.0`)
	switch datastreamType.Str {
	case "logs":
		if datastreamDataset.Str == "apm.error" {
			return errorEventType
		}
		return logEventType
	case "metrics":
		return metricEventType
	case "traces":
		if gjson.GetBytes(fields, `span\.type`).Exists() {
			return spanEventType
		}
		if gjson.GetBytes(fields, `transaction\.type`).Exists() {
			return transactionEventType
		}
	}
	return undefinedEventType
}

var docSortFields = []string{
	"trace.id",
	"transaction.id",
	"span.id",
	"error.id",
	"transaction.name",
	"span.destination.service.resource",
	"transaction.type",
	"span.type",
	"service.name",
	"service.environment",
	"message",
	"metricset.interval", // useful to sort different interval metric sets.
	"@timestamp",         // last resort before _id; order is generally guaranteed
}

func compareDocumentFields(i, j json.RawMessage) int {
	// NOTE(axw) we should remove this event type derivation and comparison
	// in the future, and sort purely on fields. We're doing this to avoid
	// reordering all the approval files while removing `processor.event`.
	// If/when we change sort order, we should add a tool for re-sorting
	// *.approved.json files.
	if n := getEventType(i) - getEventType(j); n != 0 {
		return int(n)
	}
	for _, field := range docSortFields {
		path := strings.ReplaceAll(field, ".", "\\.")
		ri := gjson.GetBytes(i, path)
		rj := gjson.GetBytes(j, path)
		if ri.Exists() && rj.Exists() {
			// 'fields' always returns an array
			// of values, but all of the fields
			// we sort on are single value fields.
			ri = ri.Array()[0]
			rj = rj.Array()[0]
		}
		if ri.Less(rj, true) {
			return -1
		}
		if rj.Less(ri, true) {
			return 1
		}
	}
	// All sort fields are equivalent, so compare bytes.
	return bytes.Compare(i, j)
}
