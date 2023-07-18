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

package tracegen

import (
	"encoding/binary"
	"math/rand"
	"time"

	"go.elastic.co/apm/v2"
)

// newRandomTraceID returns randomly generated apm.TraceID
// which is also compatible with otel's trace.TraceID
func newRandomTraceID() apm.TraceID {
	var traceID apm.TraceID
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	binary.LittleEndian.PutUint64(traceID[:8], r.Uint64())
	binary.LittleEndian.PutUint64(traceID[8:], r.Uint64())
	return traceID
}
