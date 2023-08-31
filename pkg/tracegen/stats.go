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

// EventStats holds client-side stats.
type EventStats struct {
	// ExceptionssSent holds the number of exception span events sent.
	ExceptionsSent int

	// LogsSent holds the number of logs sent, including standalone
	// log records and non-exception span events.
	LogsSent int

	// SpansSent holds the number of transactions and spans sent.
	SpansSent int
}

// Add adds the statistics together, returning the result.
func (lhs EventStats) Add(rhs EventStats) EventStats {
	return EventStats{
		ExceptionsSent: rhs.ExceptionsSent,
		LogsSent:       rhs.LogsSent,
		SpansSent:      rhs.SpansSent,
	}
}
