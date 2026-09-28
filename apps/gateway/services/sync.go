package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/internal/db/sync"
)

var ErrInvalidSyncRequest = errors.New("invalid sync request")

const ackSeparator = "|"

type SyncSender func(dtos.SyncStreamLine) error

type syncCursor map[dtos.SyncEntityType]uuid.UUID

type SyncStream struct {
	Request dtos.SyncRequestType
	Upserts dtos.SyncEntityType
	Deletes dtos.SyncEntityType
	// Every version of a resource reads the same model, which is how two versions of one resource are caught in a request.
	Model reflect.Type

	run func(ctx context.Context, repo *repositories.SyncRepository, cursor syncCursor, nowID uuid.UUID, send SyncSender) error
}

// Parents come before children so a client can apply lines in the order they arrive.
var SyncStreams = []SyncStream{
	NewSyncStream(dtos.SyncRequestTypes.RoutesV1, dtos.SyncEntityTypes.RouteV1, dtos.SyncEntityTypes.RouteDeleteV1, dtos.NewSyncRouteV1, dtos.NewSyncRouteDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.DirectionsV1, dtos.SyncEntityTypes.DirectionV1, dtos.SyncEntityTypes.DirectionDeleteV1, dtos.NewSyncDirectionV1, dtos.NewSyncDirectionDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.StopsV1, dtos.SyncEntityTypes.StopV1, dtos.SyncEntityTypes.StopDeleteV1, dtos.NewSyncStopV1, dtos.NewSyncStopDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.DirectionStopsV1, dtos.SyncEntityTypes.DirectionStopV1, dtos.SyncEntityTypes.DirectionStopDeleteV1, dtos.NewSyncDirectionStopV1, dtos.NewSyncDirectionStopDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.AlertsV1, dtos.SyncEntityTypes.AlertV1, dtos.SyncEntityTypes.AlertDeleteV1, dtos.NewSyncAlertV1, dtos.NewSyncAlertDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.AlertDirectionsV1, dtos.SyncEntityTypes.AlertDirectionV1, dtos.SyncEntityTypes.AlertDirectionDeleteV1, dtos.NewSyncAlertDirectionV1, dtos.NewSyncAlertDirectionDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.StopSchedulesV1, dtos.SyncEntityTypes.StopScheduleV1, dtos.SyncEntityTypes.StopScheduleDeleteV1, dtos.NewSyncStopScheduleV1, dtos.NewSyncStopScheduleDeleteV1),
}

func NewSyncStream[M, A sync.Row, U, D any](
	request dtos.SyncRequestType,
	upserts, deletes dtos.SyncEntityType,
	toUpsert func(M) U,
	toDelete func(A) D,
) SyncStream {
	return SyncStream{
		Request: request,
		Upserts: upserts,
		Deletes: deletes,
		Model:   reflect.TypeFor[M](),
		run: func(ctx context.Context, repo *repositories.SyncRepository, cursor syncCursor, nowID uuid.UUID, send SyncSender) error {
			for row, err := range repositories.Deletes[A](ctx, repo, cursor[deletes], nowID) {
				if err != nil {
					return err
				}
				if err := send(syncLine(deletes, row.SyncID(), toDelete(row))); err != nil {
					return err
				}
			}
			for row, err := range repositories.Upserts[M](ctx, repo, cursor[upserts], nowID) {
				if err != nil {
					return err
				}
				if err := send(syncLine(upserts, row.SyncID(), toUpsert(row))); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

type SyncPlan struct {
	streams  []SyncStream
	cursor   syncCursor
	complete uuid.UUID
}

func (p SyncPlan) Requests() []dtos.SyncRequestType {
	requests := make([]dtos.SyncRequestType, 0, len(p.streams))
	for _, s := range p.streams {
		requests = append(requests, s.Request)
	}
	return requests
}

type SyncService struct {
	repo *repositories.SyncRepository
}

func NewSyncService(repo *repositories.SyncRepository) *SyncService {
	return &SyncService{repo: repo}
}

// Plan validates a request before any line is written, so every error it returns is the client's.
func (s *SyncService) Plan(req dtos.SyncRequest) (SyncPlan, error) {
	streams, err := planStreams(req.Types)
	if err != nil {
		return SyncPlan{}, err
	}
	cursor, err := parseAcks(req.Acks)
	if err != nil {
		return SyncPlan{}, err
	}
	return SyncPlan{
		streams:  streams,
		cursor:   cursor,
		complete: cursor[dtos.SyncEntityTypes.SyncCompleteV1],
	}, nil
}

func (s *SyncService) Stream(ctx context.Context, plan SyncPlan, send SyncSender) error {
	nowID, err := s.repo.NowID(ctx)
	if err != nil {
		return err
	}

	if plan.complete != uuid.Nil && sync.Expired(plan.complete, time.Now()) {
		return send(syncLine(dtos.SyncEntityTypes.SyncResetV1, nowID, dtos.SyncResetV1{}))
	}

	for _, stream := range plan.streams {
		if err := stream.run(ctx, s.repo, plan.cursor, nowID, send); err != nil {
			return fmt.Errorf("stream %s: %w", stream.Request, err)
		}
	}

	return send(syncLine(dtos.SyncEntityTypes.SyncCompleteV1, nowID, dtos.SyncCompleteV1{}))
}

func planStreams(types []dtos.SyncRequestType) ([]SyncStream, error) {
	if len(types) == 0 {
		return nil, fmt.Errorf("%w: no types requested", ErrInvalidSyncRequest)
	}
	for _, t := range types {
		if !slices.ContainsFunc(SyncStreams, func(s SyncStream) bool { return s.Request == t }) {
			return nil, fmt.Errorf("%w: unknown type %q", ErrInvalidSyncRequest, t)
		}
	}

	planned := slices.DeleteFunc(slices.Clone(SyncStreams), func(s SyncStream) bool {
		return !slices.Contains(types, s.Request)
	})

	requested := map[reflect.Type]dtos.SyncRequestType{}
	for _, s := range planned {
		if other, ok := requested[s.Model]; ok {
			return nil, fmt.Errorf("%w: %s and %s are versions of the same resource", ErrInvalidSyncRequest, other, s.Request)
		}
		requested[s.Model] = s.Request
	}
	return planned, nil
}

// A client may send several acks for one type; the newest wins.
func parseAcks(acks []string) (syncCursor, error) {
	cursor := syncCursor{}
	for _, ack := range acks {
		entity, id, err := ParseAck(ack)
		if err != nil {
			return nil, err
		}
		if current := cursor[entity]; bytes.Compare(id[:], current[:]) > 0 {
			cursor[entity] = id
		}
	}
	return cursor, nil
}

func ParseAck(ack string) (dtos.SyncEntityType, uuid.UUID, error) {
	raw, rawID, ok := strings.Cut(ack, ackSeparator)
	if !ok {
		return "", uuid.Nil, fmt.Errorf("%w: ack %q has no %q", ErrInvalidSyncRequest, ack, ackSeparator)
	}

	entity := dtos.SyncEntityType(raw)
	if !slices.Contains(dtos.SyncEntityType("").EnumValues(), any(entity)) {
		return "", uuid.Nil, fmt.Errorf("%w: ack %q has unknown type", ErrInvalidSyncRequest, ack)
	}

	id, err := uuid.Parse(rawID)
	if _, minted := sync.MintedAt(id); err != nil || !minted {
		return "", uuid.Nil, fmt.Errorf("%w: ack %q does not end in a uuidv7", ErrInvalidSyncRequest, ack)
	}
	return entity, id, nil
}

func FormatAck(entity dtos.SyncEntityType, id uuid.UUID) string {
	return string(entity) + ackSeparator + id.String()
}

func syncLine(entity dtos.SyncEntityType, id uuid.UUID, data any) dtos.SyncStreamLine {
	return dtos.SyncStreamLine{Type: entity, Ack: FormatAck(entity, id), Data: data}
}
