package main

import (
	"fmt"

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) envCommand(c *cli.Context) error {
	creds, err := cmd.getCredentials(c)
	if err != nil {
		return err
	}

	fmt.Printf("export ELASTIC_APM_SERVER_URL=%q;\n", cmd.cfg.APMServerURL)
	if creds.SecretToken != "" {
		fmt.Printf("export ELASTIC_APM_SECRET_TOKEN=%q;\n", creds.SecretToken)
	} else if creds.APIKey != "" {
		fmt.Printf("export ELASTIC_APM_API_KEY=%q;\n", creds.APIKey)
	}

	fmt.Printf("export OTEL_EXPORTER_OTLP_ENDPOINT=%q;\n", cmd.cfg.APMServerURL)
	if creds.SecretToken != "" {
		fmt.Printf("export OTEL_EXPORTER_OTLP_HEADERS=%q;\n", "Authorization=Bearer "+creds.SecretToken)
	} else if creds.APIKey != "" {
		fmt.Printf("export OTEL_EXPORTER_OTLP_HEADERS=%q;\n", "Authorization=ApiKey "+creds.APIKey)
	}

	return nil
}

// NewPrintEnvCmd prints environment variables for configuring Elastic APM agent
func NewPrintEnvCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "agent-env",
		Usage:  "print environment variables for configuring Elastic APM agents",
		Action: commands.envCommand,
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:  "api-key-expiration",
				Usage: "specify how long before a created API Key expires. 0 means it never expires.",
			},
		},
	}
}
