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
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) servicesCommand(ctx context.Context, c *cli.Command) error {
	client, err := cmd.getClient()
	if err != nil {
		return err
	}
	services, err := client.ServiceSummary(ctx)
	if err != nil {
		return err
	}
	for _, service := range services {
		fmt.Println(service)
	}
	return nil
}

// NewListServiceCmd returns pointer to a Command that talks to APM Server and list all APM services
func NewListServiceCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "list-services",
		Usage:  "list APM services",
		Action: commands.servicesCommand,
	}
}
