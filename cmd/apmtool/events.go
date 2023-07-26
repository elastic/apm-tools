package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) sendEventsCommand(c *cli.Context) error {
	creds, err := cmd.getCredentials(c)
	if err != nil {
		return err
	}

	var body io.Reader
	filename := c.String("file")
	if filename == "-" {
		body = io.NopCloser(os.Stdin)
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("error opening file: %w", err)
		}
		defer f.Close()
		body = f
	}

	urlPath := "/intake/v2/events"
	if c.Bool("rumv2") {
		urlPath = "/intake/v2/rum/events"
	}
	req, err := http.NewRequest(
		http.MethodPost,
		cmd.cfg.APMServerURL+urlPath+"?verbose",
		body,
	)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")

	switch {
	case creds.SecretToken != "":
		req.Header.Set("Authorization", "Bearer "+creds.SecretToken)
	case creds.APIKey != "":
		req.Header.Set("Authorization", "ApiKey "+creds.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error performing HTTP request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stderr, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("error sending events; server responded with %q", resp.Status)
	}
	return nil
}

func NewSendEventCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "send-events",
		Usage:  "send events stored in ND-JSON format",
		Action: commands.sendEventsCommand,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Required: true,
				Usage:    "File containing the payload to send, in ND-JSON format. Use '-' to read from stdin.",
			},
			&cli.BoolFlag{
				Name:  "rumv2",
				Usage: "Send events to /intake/v2/rum/events",
			},
		},
	}
}
