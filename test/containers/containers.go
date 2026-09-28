// Package containers runs the throwaway Postgres the medium specs talk to, and provisions a fresh database per spec.
package containers

import "context"

// Container is a dockerized backend. Start returns a connection string that is safe to send to Ginkgo's parallel processes.
type Container interface {
	Start(ctx context.Context) (string, error)
	Stop(ctx context.Context) error
}
