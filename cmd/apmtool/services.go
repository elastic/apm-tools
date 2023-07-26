package main

import (
	"fmt"

	"github.com/urfave/cli/v3"
)

func (cmd *Commands) servicesCommand(c *cli.Context) error {
	client, err := cmd.getClient()
	if err != nil {
		return err
	}
	services, err := client.ServiceSummary(c.Context)
	if err != nil {
		return err
	}
	for _, service := range services {
		fmt.Println(service)
	}
	return nil
}

func NewListServiceCmd(commands *Commands) *cli.Command {
	return &cli.Command{
		Name:   "list-services",
		Usage:  "list APM services",
		Action: commands.servicesCommand,
	}
}
