package main

import (
	"fmt"
	"os"

	"github.com/odit-services/cnpg-plugin-pgdump/cmd/plugin"
)

var version = "dev"

func main() {
	if err := plugin.New(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
