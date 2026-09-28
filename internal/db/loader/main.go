// Command loader prints the desired schema for atlas to diff against. It exists
// because atlas reads an external schema as DDL on stdout.
package main

import (
	"fmt"
	"io"
	"os"

	"ariga.io/atlas-provider-bun/bunschema"
	_ "ariga.io/atlas/sdk/recordriver"

	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/sync"
)

func main() {
	models := append(sync.Models(model.SyncTables), model.ServerTables...)

	stmts, err := bunschema.New(bunschema.DialectPostgres).Load(models...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load bun schema: %v\n", err)
		os.Exit(1)
	}
	io.WriteString(os.Stdout, stmts)
	io.WriteString(os.Stdout, sync.IndexDDL(model.SyncTables))
}
