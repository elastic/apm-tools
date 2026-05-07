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

// YAML output is produced by byte-level splicing rather than via a YAML
// library. Goal: keep every byte outside the modified subtree exactly as it
// appears in the input so the output is byte-identical to the upstream
// Python script (which uses ruamel.yaml). Round-tripping the whole document
// through any Go YAML library introduces formatting drift (whitespace,
// quoting, blank lines after empty folded scalars) that's painful to chase.
// stdlib has no YAML support, so the trade-off is between rolling our own
// parser or accepting drift; the modifications we need are small and the
// file shape is constrained, so a small line-based splicer is the simpler
// choice.
//
// The files we edit are pure block-style YAML written with a consistent
// 4-column indent per nesting level (matching ruamel.yaml's mapping=2,
// sequence=4, offset=2 emitter after the script's "strip 2 leading spaces
// from each line" post-process). All three target files use only "- name:"
// or "- key:" entries to start block-sequence items, no tabs, no anchors,
// and no flow style at the levels we navigate. Those constraints keep the
// scanner tiny.

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// modifyBeatRoot updates metricbeat/module/beat/_meta/fields.yml.
//
// Layout:
//
//   - key: beat
//     fields:
//   - name: beats_stats
//     fields:
//   - name: apm-server         <-- upsert target indent = 8
//
// Aliases are produced for each metric.
func modifyBeatRoot(path string, stats []byte) error {
	return upsertYAML(path, stats, yamlPlan{
		path: []yamlPathStep{
			{key: "key", value: "beat"},
			{key: "name", value: "beats_stats"},
		},
		childIndent:   8,
		alias:         true,
		nameTransform: identityName, // beats_stats children keep hyphens
	})
}

// modifyBeatStats updates metricbeat/module/beat/stats/_meta/fields.yml.
//
// Layout:
//
//   - name: stats
//     fields:
//   - name: apm_server          <-- upsert target indent = 4
func modifyBeatStats(path string, stats []byte) error {
	return upsertYAML(path, stats, yamlPlan{
		path:          []yamlPathStep{{key: "name", value: "stats"}},
		childIndent:   4,
		alias:         false,
		nameTransform: underscoreName,
	})
}

// modifyEAFields updates the integrations beat-fields.yml.
//
// Layout:
//
//   - name: beat.stats
//     fields:
//   - name: apm_server          <-- upsert target indent = 4
//
// The Elastic Agent integration package is installed onto a TSDB data stream
// (manifest.yml: elasticsearch.index_mode: time_series). On a TSDB stream
// each numeric field needs a metric_type: gauge|counter annotation, which
// Fleet/EPM translates into the time_series_metric mapping parameter; that
// parameter drives downsampling semantics. /stats carries values only, so
// this tool cannot derive gauge vs counter from the JSON. The EA file thus
// has metricType: retain — existing annotations are carried forward by name
// match, and brand-new fields are emitted with metric_type: FIXME so a
// human has to choose before the regen output is committed upstream.
func modifyEAFields(path string, stats []byte) error {
	return upsertYAML(path, stats, yamlPlan{
		path:          []yamlPathStep{{key: "name", value: "beat.stats"}},
		childIndent:   4,
		alias:         false,
		nameTransform: underscoreName,
		metricType:    metricTypeRetain,
	})
}

type yamlPlan struct {
	// path is a sequence of (mapping key, value) pairs. Starting from the
	// top-level block sequence, each step locates the entry whose mapping
	// key field equals value, then descends into that entry's "fields:"
	// list. The final fields list is the upsert target.
	path []yamlPathStep

	// childIndent is the column of the dash for direct children of the
	// target fields list (after the script's leading-2-spaces strip).
	childIndent int

	// alias selects between alias-style and concrete-type output.
	alias bool

	// nameTransform converts a metric key (e.g. "apm-server") into the YAML
	// name used inside the file (e.g. "apm_server" for beat.stats variants).
	nameTransform func(string) string

	// metricType controls whether typed leaves carry a metric_type:
	// annotation. metricTypeOmit (the default) leaves the annotation off
	// entirely. metricTypeRetain copies the annotation from the existing
	// entry at the same dotted path, or emits "FIXME" for brand-new fields.
	metricType metricTypePolicy
}

type metricTypePolicy int

const (
	metricTypeOmit metricTypePolicy = iota
	metricTypeRetain
)

type yamlPathStep struct {
	key   string // "key" or "name"
	value string
}

func identityName(s string) string   { return s }
func underscoreName(s string) string { return strings.ReplaceAll(s, "-", "_") }

// upsertYAML applies plan to path. It locates the target fields list by
// scanning the source as text, modifies that list, and writes the result.
// Bytes outside the target list are passed through unchanged.
func upsertYAML(path string, stats []byte, plan yamlPlan) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	listStart, listEnd, err := locateFieldsList(src, plan.path, plan.childIndent)
	if err != nil {
		return fmt.Errorf("locating target list in %s: %w", path, err)
	}

	items := splitListItems(src[listStart:listEnd], plan.childIndent)
	for _, m := range metrics {
		yamlName := plan.nameTransform(m.name)
		fields, err := fieldsYAML(stats, m, plan.alias)
		if err != nil {
			return err
		}
		if fields == nil {
			warnMissing(m, path)
			continue
		}
		// When metricType: retain, look at the existing entry being
		// replaced and carry forward its metric_type annotations by
		// dotted path. New fields get metric_type: FIXME.
		var existingMetricTypes map[string]string
		var existingFields []item
		for _, it := range items {
			if it.name != yamlName {
				continue
			}
			if plan.metricType == metricTypeRetain {
				existingMetricTypes = extractMetricTypes(it.body)
			}
			existingFields = parseEntryFields(it.body)
			break
		}
		merged := mergeFields(existingFields, fields)
		var rendered bytes.Buffer
		renderItem(&rendered, item{Name: yamlName, Type: "group", Fields: merged},
			plan.childIndent, plan.metricType, existingMetricTypes, "")
		items = upsertListItem(items, yamlName, rendered.Bytes())
	}

	var out bytes.Buffer
	out.Write(src[:listStart])
	for _, it := range items {
		out.Write(it.body)
	}
	out.Write(src[listEnd:])
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// listItem is one block-sequence entry parsed from the source bytes. body
// contains the entry's literal bytes including its trailing newline.
type listItem struct {
	name string
	body []byte
}

// splitListItems splits the bytes of a block-sequence list into entries.
// Each entry begins at "<indent>- " and ends just before the next sibling
// (or at the end of b).
func splitListItems(b []byte, indent int) []listItem {
	dash := []byte(strings.Repeat(" ", indent) + "- ")
	var items []listItem
	for start := 0; start < len(b); {
		end := nextLineWithPrefix(b, start+1, dash)
		if end < 0 {
			end = len(b)
		}
		items = append(items, listItem{
			name: extractItemName(b[start:end]),
			body: append([]byte(nil), b[start:end]...),
		})
		start = end
	}
	return items
}

// extractItemName returns the value of the "name:" field on the first line
// of an entry. The target lists never contain "key:" entries (only the
// top-level navigation does), so we don't accept that form here.
func extractItemName(entry []byte) string {
	line, _, _ := bytes.Cut(entry, []byte("\n"))
	s := string(bytes.TrimLeft(line, " "))
	const prefix = "- name: "
	if !strings.HasPrefix(s, prefix) {
		return ""
	}
	return unquoteYAMLScalar(strings.TrimSpace(s[len(prefix):]))
}

// unquoteYAMLScalar strips a single layer of matching double or single quotes
// from s. It does not interpret YAML escape sequences; the only quoted
// scalars the upsert path encounters are simple identifiers.
func unquoteYAMLScalar(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// upsertListItem replaces the entry whose name matches replacement's name,
// or appends if none matches. replacement holds the literal bytes of the
// new entry, including the trailing newline.
func upsertListItem(items []listItem, name string, replacement []byte) []listItem {
	body := append([]byte(nil), replacement...)
	for i := range items {
		if items[i].name == name {
			items[i].body = body
			return items
		}
	}
	return append(items, listItem{name: name, body: body})
}

// renderItem emits a YAML block-sequence entry for it at the given dash
// column. The indentation pattern (children at +4) mirrors the post-strip
// output of ruamel.yaml configured with mapping=2, sequence=4, offset=2.
//
// When mt == metricTypeRetain, typed leaves are followed by a metric_type:
// line whose value is looked up by dotted path in retain (the snapshot of
// the existing entry being replaced). Leaves with no prior annotation get
// "FIXME". groups and aliases never carry metric_type. parentPath is the
// dotted path of the parent group, "" for the top-level entry.
func renderItem(buf *bytes.Buffer, it item, indent int, mt metricTypePolicy, retain map[string]string, parentPath string) {
	pad := strings.Repeat(" ", indent)
	cont := strings.Repeat(" ", indent+2)
	fmt.Fprintf(buf, "%s- name: %s\n", pad, it.Name)
	fullPath := it.Name
	if parentPath != "" {
		fullPath = parentPath + "." + it.Name
	}
	switch it.Type {
	case "alias":
		fmt.Fprintf(buf, "%stype: alias\n", cont)
		fmt.Fprintf(buf, "%spath: %s\n", cont, it.Path)
	case "group":
		fmt.Fprintf(buf, "%stype: group\n", cont)
		fmt.Fprintf(buf, "%sfields:\n", cont)
		for _, child := range it.Fields {
			renderItem(buf, child, indent+4, mt, retain, fullPath)
		}
	default:
		fmt.Fprintf(buf, "%stype: %s\n", cont, it.Type)
		if mt == metricTypeRetain {
			value, ok := retain[fullPath]
			if !ok {
				value = "FIXME"
			}
			fmt.Fprintf(buf, "%smetric_type: %s\n", cont, value)
		}
	}
}

// yamlEntry mirrors the schema this tool reads on the upstream files
// and writes back: each entry has a name, type, optional alias path,
// optional fields list, and optional metric_type annotation. yaml.v3
// handles every layout concern (indent, folded scalars, comments,
// quoting variants) so this parser doesn't depend on the file's
// formatting being any particular shape.
type yamlEntry struct {
	Name       string      `yaml:"name"`
	Type       string      `yaml:"type"`
	Path       string      `yaml:"path"`
	Fields     []yamlEntry `yaml:"fields"`
	MetricType string      `yaml:"metric_type"`
}

// parseEntryFields parses the bytes of a single block-sequence item
// (the YAML chunk between two list-item dashes) and returns the
// []item tree for that entry's nested "fields:" list. Returns nil for
// leaf or alias entries (no nested fields) and on parse errors.
//
// Goes through gopkg.in/yaml.v3, so the parse is robust against any
// formatting the upstream files use — folded scalars, multi-line
// strings, in-line comments, alternate quote styles — none of those
// affect the result.
func parseEntryFields(entry []byte) []item {
	var seq []yamlEntry
	if err := yaml.Unmarshal(entry, &seq); err != nil || len(seq) == 0 {
		return nil
	}
	return entriesToItems(seq[0].Fields)
}

func entriesToItems(entries []yamlEntry) []item {
	if len(entries) == 0 {
		return nil
	}
	out := make([]item, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		out = append(out, item{
			Name:   e.Name,
			Type:   e.Type,
			Path:   e.Path,
			Fields: entriesToItems(e.Fields),
		})
	}
	return out
}

// extractMetricTypes returns a map of dotted leaf path to metric_type
// value for every typed (non-group, non-alias) leaf in entry that
// carried a metric_type: annotation. Group and alias entries are
// skipped because metric_type only applies to numeric leaves.
//
// Paths include the root entry's name as the leading component, to
// match the fullPath renderItem builds when emitting the merged tree.
func extractMetricTypes(entry []byte) map[string]string {
	var seq []yamlEntry
	if err := yaml.Unmarshal(entry, &seq); err != nil || len(seq) == 0 {
		return nil
	}
	out := map[string]string{}
	collectMetricTypes(seq[0].Fields, seq[0].Name, out)
	return out
}

func collectMetricTypes(entries []yamlEntry, prefix string, out map[string]string) {
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		path := e.Name
		if prefix != "" {
			path = prefix + "." + e.Name
		}
		if len(e.Fields) > 0 {
			collectMetricTypes(e.Fields, path, out)
			continue
		}
		if e.Type == "alias" {
			continue
		}
		if e.MetricType != "" {
			out[path] = e.MetricType
		}
	}
}

// locateFieldsList walks plan.path through the document and returns the
// half-open byte range of the target fields list's children — that is,
// from the first child's dash to (but not including) the byte after the
// last child.
func locateFieldsList(src []byte, path []yamlPathStep, childIndent int) (int, int, error) {
	dashCol := 0
	pos := 0
	for i, step := range path {
		entryStart, entryEnd, err := findEntry(src, pos, dashCol, step)
		if err != nil {
			return 0, 0, fmt.Errorf("step %d (%s=%s): %w", i, step.key, step.value, err)
		}
		// "fields:" sits one mapping-indent level inside the entry: column
		// dashCol+2. Locate it and skip past the line.
		fieldsCol := dashCol + 2
		fieldsRel := nextLineWithPrefix(src[entryStart:entryEnd], 0, []byte(strings.Repeat(" ", fieldsCol)+"fields:"))
		if fieldsRel < 0 {
			return 0, 0, fmt.Errorf("step %d (%s=%s): fields: not found at column %d", i, step.key, step.value, fieldsCol)
		}
		nl := bytes.IndexByte(src[entryStart+fieldsRel:], '\n')
		if nl < 0 {
			return 0, 0, fmt.Errorf("step %d: 'fields:' line has no newline", i)
		}
		pos = entryStart + fieldsRel + nl + 1
		dashCol += 4
	}
	if dashCol != childIndent {
		return 0, 0, fmt.Errorf("computed dash column %d does not match expected childIndent %d", dashCol, childIndent)
	}
	return pos, scanListEnd(src, pos, childIndent), nil
}

// findEntry locates a block-sequence entry whose dash sits at column dashCol
// and whose first mapping pair is step.key: step.value. It returns the
// half-open byte range of the entry, from its dash to the start of the next
// sibling (or end of the parent block).
func findEntry(src []byte, from, dashCol int, step yamlPathStep) (int, int, error) {
	prefix := []byte(strings.Repeat(" ", dashCol) + "- " + step.key + ": ")
	for i := from; i < len(src); {
		j := nextLineWithPrefix(src, i, prefix)
		if j < 0 {
			break
		}
		// Compare the value (up to end of line).
		valStart := j + len(prefix)
		nl := bytes.IndexByte(src[valStart:], '\n')
		if nl < 0 {
			nl = len(src) - valStart
		}
		got := unquoteYAMLScalar(strings.TrimSpace(string(src[valStart : valStart+nl])))
		if got == step.value {
			return j, scanListEnd(src, valStart+nl+1, dashCol), nil
		}
		i = valStart + nl + 1
	}
	return 0, 0, fmt.Errorf("entry %s=%s not found at column %d", step.key, step.value, dashCol)
}

// nextLineWithPrefix returns the byte offset of the next line, at or after
// from, that begins with prefix. Returns -1 if none.
func nextLineWithPrefix(src []byte, from int, prefix []byte) int {
	for i := from; i < len(src); i++ {
		if i > 0 && src[i-1] != '\n' {
			continue
		}
		if bytes.HasPrefix(src[i:], prefix) {
			return i
		}
	}
	return -1
}

// scanListEnd returns the byte offset of the first line at or after lineStart
// whose first non-space character sits at a column strictly less than
// dashCol. lineStart must be aligned to a line boundary. Blank lines are
// treated as part of the list.
func scanListEnd(src []byte, lineStart, dashCol int) int {
	for i := lineStart; i < len(src); {
		nl := bytes.IndexByte(src[i:], '\n')
		var lineEnd int
		if nl < 0 {
			lineEnd = len(src)
		} else {
			lineEnd = i + nl + 1
		}
		col := 0
		for col < lineEnd-i && src[i+col] == ' ' {
			col++
		}
		// Treat blank lines (only whitespace before newline/EOF) as in-list.
		isBlank := col == lineEnd-i || src[i+col] == '\n'
		if !isBlank && col < dashCol {
			return i
		}
		if nl < 0 {
			return lineEnd
		}
		i = lineEnd
	}
	return len(src)
}
