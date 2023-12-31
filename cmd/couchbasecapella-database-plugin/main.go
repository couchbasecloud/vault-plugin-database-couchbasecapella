package main

import (
	"os"

	couchbasecapella "github.com/couchbasecloud/vault-plugin-database-couchbasecapella"
	hclog "github.com/hashicorp/go-hclog"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

func main() {
	err := Run()
	if err != nil {
		logger := hclog.New(&hclog.LoggerOptions{})

		logger.Error("plugin shutting down", "error", err)
		os.Exit(1)
	}
}

// Run instantiates a CouchbaseDB object, and runs the RPC server for the plugin
func Run() error {
	dbplugin.ServeMultiplex(couchbasecapella.New)

	return nil
}
