package services

import (
	"context"
	"fmt"
	"time"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/internal/db/sync"
)

const InactiveRouteRetention = 14 * 24 * time.Hour

type DatabaseCleanupService struct {
	repo *repositories.RouteDataRepository
}

func NewDatabaseCleanupService(repo *repositories.RouteDataRepository) *DatabaseCleanupService {
	return &DatabaseCleanupService{repo: repo}
}

func (s *DatabaseCleanupService) Cleanup(ctx context.Context) error {
	if err := s.repo.CleanupInactiveRoutes(ctx, InactiveRouteRetention); err != nil {
		return fmt.Errorf("cleaning up inactive routes: %w", err)
	}

	if err := s.repo.CleanupSyncTombstones(ctx, sync.TombstoneRetention); err != nil {
		return fmt.Errorf("cleaning up sync tombstones: %w", err)
	}

	return nil
}
