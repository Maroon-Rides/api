package containers

import (
	"context"
	"fmt"
	"net/url"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/MaroonRides/api/internal/db"
)

const (
	image         = "postgres:18-alpine"
	port          = "5432/tcp"
	adminDatabase = "postgres"
	credentials   = "postgres"
	readyLog      = "database system is ready to accept connections"
	// Postgres logs readiness once for the init server and once for the real one.
	readyLogCount  = 2
	startupTimeout = 60 * time.Second
)

// TemplateDatabase is migrated once per run; every spec database is a copy of it.
const TemplateDatabase = "template_maroonrides"

type Postgres struct {
	container tc.Container
}

func (p *Postgres) Start(ctx context.Context) (string, error) {
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		Image:        image,
		ExposedPorts: []string{port},
		Env: map[string]string{
			"POSTGRES_DB":       adminDatabase,
			"POSTGRES_USER":     credentials,
			"POSTGRES_PASSWORD": credentials,
		},
		// These databases are thrown away, so trade crash safety for speed.
		Cmd: []string{
			"postgres",
			"-c", "fsync=off",
			"-c", "full_page_writes=off",
			"-c", "synchronous_commit=off",
		},
		WaitingFor: wait.ForLog(readyLog).WithOccurrence(readyLogCount).WithStartupTimeout(startupTimeout),
		Started:    true,
	})
	if err != nil {
		return "", err
	}
	p.container = c

	host, err := c.Host(ctx)
	if err != nil {
		return "", err
	}
	mapped, err := c.MappedPort(ctx, port)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", credentials, credentials, host, mapped.Port(), adminDatabase), nil
}

func (p *Postgres) Stop(_ context.Context) error {
	if p.container == nil {
		return nil
	}
	return tc.TerminateContainer(p.container)
}

// CreateTemplateDatabase returns a DSN for an empty template. The caller must migrate it and close
// every connection before the first clone, since Postgres refuses to copy a database in use.
func CreateTemplateDatabase(adminDSN string) string {
	admin(adminDSN, fmt.Sprintf("CREATE DATABASE %q", TemplateDatabase))
	return WithDatabase(adminDSN, TemplateDatabase)
}

func CreateDatabase(adminDSN, name string) string {
	admin(adminDSN, fmt.Sprintf("CREATE DATABASE %q WITH TEMPLATE %q", name, TemplateDatabase))
	return WithDatabase(adminDSN, name)
}

func DropDatabase(adminDSN, name string) {
	admin(adminDSN, fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name))
}

func WithDatabase(dsn, name string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		panic(err)
	}
	u.Path = "/" + name
	return u.String()
}

func admin(adminDSN, statement string) {
	conn, err := db.Open(adminDSN)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(statement); err != nil {
		panic(fmt.Errorf("%s: %w", statement, err))
	}
}
