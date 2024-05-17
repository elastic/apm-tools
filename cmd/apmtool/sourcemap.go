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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) uploadSourcemapCommand(ctx context.Context, c *cli.Command) error {
	var data bytes.Buffer
	mw := multipart.NewWriter(&data)
	mw.WriteField("service_name", c.String("service-name"))
	mw.WriteField("service_version", c.String("service-version"))
	mw.WriteField("bundle_filepath", c.String("bundle-filepath"))
	sourcemapFileWriter, err := mw.CreateFormFile("sourcemap", "sourcemap.js.map")
	if err != nil {
		return err
	}
	if filename := c.String("file"); filename == "" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			log.Fatalf("failed to stat stdin: %s", err.Error())
		}
		if stat.Size() == 0 {
			log.Fatal("empty -file flag and stdin, please set one.")
		}
		if _, err := io.Copy(sourcemapFileWriter, os.Stdin); err != nil {
			return err
		}
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("error opening file: %w", err)
		}
		if _, err := io.Copy(sourcemapFileWriter, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	if err := mw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		cmd.cfg.KibanaURL+"/api/apm/sourcemaps",
		bytes.NewReader(data.Bytes()),
	)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.SetBasicAuth(cmd.cfg.Username, cmd.cfg.Password)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("kbn-xsrf", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error performing HTTP request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stderr, resp.Body)
	fmt.Fprintln(os.Stderr)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error uploading sourcemap; server responded with %q", resp.Status)
	}
	return nil
}

// NewUploadSourcemapCmd returns pointer to a Command that uploads a source map to Kibana
func NewUploadSourcemapCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "upload-sourcemap",
		Usage:  "upload a source map to Kibana",
		Action: commands.uploadSourcemapCommand,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Required: true,
				Usage:    "File containing the sourcemap to upload. Sourcemap must be provided via this flag or stdin.",
			},
			&cli.StringFlag{
				Name:     "service-name",
				Required: true,
				Usage:    "service.name value to match against events",
			},
			&cli.StringFlag{
				Name:     "service-version",
				Required: true,
				Usage:    "service.version value to match against events",
			},
			&cli.StringFlag{
				Name:     "bundle-filepath",
				Required: true,
				Usage:    "Source bundle filepath to match against stack frames locations.",
			},
		},
	}
}
