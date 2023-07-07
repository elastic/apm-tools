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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	// ApprovedSuffix signals a file has been reviewed and approved.
	ApprovedSuffix = ".approved.json"

	// ReceivedSuffix signals a file has changed and not yet been approved.
	ReceivedSuffix = ".received.json"
)

// ApproveEventDocs compares the given event documents with
// the contents of the file in "<name>.approved.json".
//
// Any specified dynamic fields (e.g. @timestamp, observer.id)
// will be replaced with a static string for comparison.
//
// If the events differ, then the test will fail.
func ApproveEventDocs(t testing.TB, name string, eventDocs [][]byte, dynamic ...string) {
	t.Helper()

	// Rewrite all dynamic fields to have a known value,
	// so dynamic fields don't affect diffs.
	events := make([]interface{}, len(eventDocs))
	for i, doc := range eventDocs {
		for _, field := range dynamic {
			existing := gjson.GetBytes(doc, field)
			if !existing.Exists() {
				continue
			}

			var err error
			doc, err = sjson.SetBytes(doc, field, "dynamic")
			if err != nil {
				t.Fatal(err)
			}
		}

		var event map[string]interface{}
		if err := json.Unmarshal(doc, &event); err != nil {
			t.Fatal(err)
		}
		events[i] = event
	}

	received := map[string]interface{}{"events": events}
	approve(t, name, received)
}

// approve compares the given value with the contents of the file
// "<name>.approved.json".
//
// If the value differs, then the test will fail.
func approve(t testing.TB, name string, received interface{}) {
	t.Helper()

	var approved interface{}
	if err := readApproved(name, &approved); err != nil {
		t.Fatalf("failed to read approved file: %v", err)
	}
	if diff := cmp.Diff(approved, received); diff != "" {
		if err := writeReceived(name, received); err != nil {
			t.Fatalf("failed to write received file: %v", err)
		}
		t.Fatalf("%s\n%s\n\n", diff,
			"Test failed. Run `make check-approvals` to verify the diff.",
		)
	} else {
		// Remove an old *.received.json file if it exists, ignore errors
		_ = removeReceived(name)
	}
}

func readApproved(name string, approved interface{}) error {
	path := name + ApprovedSuffix
	f, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to open approved file for %s: %w", name, err)
	}
	defer f.Close()
	if os.IsNotExist(err) {
		return nil
	}
	if err := json.NewDecoder(f).Decode(&approved); err != nil {
		return fmt.Errorf("failed to decode approved file for %s: %w", name, err)
	}
	return nil
}

func removeReceived(name string) error {
	return os.Remove(name + ReceivedSuffix)
}

func writeReceived(name string, received interface{}) error {
	fullpath := name + ReceivedSuffix
	if err := os.MkdirAll(filepath.Dir(fullpath), 0755); err != nil {
		return fmt.Errorf("failed to create directories for received file: %w", err)
	}
	f, err := os.Create(fullpath)
	if err != nil {
		return fmt.Errorf("failed to create received file for %s: %w", name, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	if err := enc.Encode(received); err != nil {
		return fmt.Errorf("failed to encode received file for %s: %w", name, err)
	}
	return nil
}
