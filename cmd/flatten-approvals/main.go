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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var inplace = flag.Bool("i", false, "modify file in place")

func main() {
	flag.Parse()
	if err := flatten(flag.Args()); err != nil {
		log.Fatal(err)
	}
}

func flatten(args []string) error {
	var filepaths []string
	for _, arg := range args {
		matches, err := filepath.Glob(arg)
		if err != nil {
			return err
		}
		filepaths = append(filepaths, matches...)
	}
	for _, filepath := range filepaths {
		if err := transform(filepath); err != nil {
			return fmt.Errorf("error transforming %q: %w", filepath, err)
		}
	}
	return nil
}

// transform []{"events": {"object": {"field": ...}}} to []{"field", "field", ...}
func transform(filepath string) error {
	var input struct {
		Events []map[string]any `json:"events"`
	}
	if err := decodeJSONFile(filepath, &input); err != nil {
		return fmt.Errorf("could not read existing approved events file: %w", err)
	}
	out := make([]map[string][]any, 0, len(input.Events))
	for _, event := range input.Events {
		fields := make(map[string][]any)
		flattenFields("", event, fields)
		out = append(out, fields)
	}

	var w io.Writer = os.Stdout
	if *inplace {
		f, err := os.Create(filepath)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	return enc.Encode(out)
}

func flattenFields(k string, v any, out map[string][]any) {
	switch v := v.(type) {
	case map[string]any:
		for k2, v := range v {
			if k != "" {
				k2 = k + "." + k2
			}
			flattenFields(k2, v, out)
		}
	case []any:
		for _, v := range v {
			flattenFields(k, v, out)
		}
	default:
		out[k] = append(out[k], v)
	}
}

func decodeJSONFile(path string, out interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return fmt.Errorf("cannot unmarshal file %q: %w", path, err)
	}
	return nil
}
