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

//go:build integration
// +build integration

package metricgen_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-tools/pkg/metricgen"
)

func TestSendOTLP_http(t *testing.T) {
	// generate these using tilt + apmtool agent-env
	u := os.Getenv("ELASTIC_APM_SERVER_URL")
	apiKey := os.Getenv("ELASTIC_APM_API_KEY")

	s, err := metricgen.SendOTLP(context.Background(),
		metricgen.WithAPMServerURL(u),
		metricgen.WithAPIKey(apiKey),
		metricgen.WithVerifyServerCert(false),
		metricgen.WithOTLPServiceName("metricgen_otlp_test"),
		metricgen.WithOTLPProtocol("http/protobuf"),
	)
	require.NoError(t, err)

	t.Logf("%+v\n", s)
	assert.Equal(t, 1, s.MetricSent)
}

func TestSendOTLP_grpc(t *testing.T) {
	// generate these using tilt + apmtool agent-env
	u := os.Getenv("ELASTIC_APM_SERVER_URL")
	apiKey := os.Getenv("ELASTIC_APM_API_KEY")

	s, err := metricgen.SendOTLP(context.Background(),
		metricgen.WithAPMServerURL(u),
		metricgen.WithAPIKey(apiKey),
		metricgen.WithVerifyServerCert(false),
		metricgen.WithOTLPServiceName("metricgen_otlp_test"),
		metricgen.WithOTLPProtocol("grpc"),
	)
	require.NoError(t, err)

	t.Logf("%+v\n", s)
	assert.Equal(t, 1, s.MetricSent)
}
