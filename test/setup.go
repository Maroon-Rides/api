package test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"

	"github.com/onsi/ginkgo/v2"
	"github.com/pressly/goose/v3"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db"
	"github.com/MaroonRides/api/test/containers"
)

var (
	pg = &containers.Postgres{}

	adminDSN string
	dbCount  atomic.Int64
)

// backends carries the connection details from Ginkgo's primary process to every parallel one.
type backends struct {
	PostgresDSN string `json:"postgresDsn"`
}

// StartBackends boots Postgres and migrates the template. Pair it with UseBackends in a SynchronizedBeforeSuite.
func StartBackends() []byte {
	ctx := context.Background()
	dsn, err := pg.Start(ctx)
	if err != nil {
		panic(err)
	}
	migrateTemplate(containers.CreateTemplateDatabase(dsn))

	data, err := json.Marshal(backends{PostgresDSN: dsn})
	if err != nil {
		panic(err)
	}
	return data
}

func migrateTemplate(dsn string) {
	goose.SetLogger(log.New(ginkgo.GinkgoWriter, "", log.LstdFlags))

	template, err := db.Open(dsn)
	if err != nil {
		panic(err)
	}
	defer template.Close()
	if err := db.Migrate(context.Background(), template); err != nil {
		panic(err)
	}
}

func UseBackends(data []byte) {
	var b backends
	if err := json.Unmarshal(data, &b); err != nil {
		panic(err)
	}
	adminDSN = b.PostgresDSN
}

func StopBackends() {
	_ = pg.Stop(context.Background())
}

// Configure gives the calling spec its own migrated database and drops it when the spec ends.
func Configure() *bun.DB {
	name := fmt.Sprintf("test_%d_%d", ginkgo.GinkgoParallelProcess(), dbCount.Add(1))
	bundb, err := db.Open(containers.CreateDatabase(adminDSN, name))
	if err != nil {
		panic(err)
	}

	ginkgo.DeferCleanup(func() {
		_ = bundb.Close()
		containers.DropDatabase(adminDSN, name)
	})
	return bundb
}
