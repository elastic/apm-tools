package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) uploadSourcemapCommand(c *cli.Context) error {
	var data bytes.Buffer
	mw := multipart.NewWriter(&data)
	mw.WriteField("service_name", c.String("service-name"))
	mw.WriteField("service_version", c.String("service-version"))
	mw.WriteField("bundle_filepath", c.String("bundle-filepath"))
	sourcemapFileWriter, err := mw.CreateFormFile("sourcemap", "sourcemap.js.map")
	if err != nil {
		return err
	}
	if filename := c.String("file"); filename == "-" {
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
				Usage:    "File containing the sourcemap to upload. Use '-' to read from stdin.",
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
