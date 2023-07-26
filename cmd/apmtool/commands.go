package main

import (
	"github.com/elastic/apm-tools/pkg/apmclient"
)

type Commands struct {
	cfg apmclient.Config
}

func (cmd *Commands) getClient() (*apmclient.Client, error) {
	return apmclient.New(cmd.cfg)
}
