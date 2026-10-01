package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
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

// syncCursor holds the newest acked id per entity type.
type syncCursor map[dtos.SyncEntityType]uuid.UUID

// SyncProtocol names the control lines a protocol version sends. A client on an
// older protocol keeps receiving the lines it knows.
type SyncProtocol struct {
	Reset    dtos.SyncEntityType
	Complete dtos.SyncEntityType
}

var SyncProtocols = map[dtos.SyncProtocolVersion]SyncProtocol{
	dtos.SyncProtocolVersions.V1: {Reset: dtos.SyncEntityTypes.SyncResetV1, Complete: dtos.SyncEntityTypes.SyncCompleteV1},
}

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
	NewSyncStream(dtos.SyncRequestTypes.TimetablesV1, dtos.SyncEntityTypes.TimetableV1, dtos.SyncEntityTypes.TimetableDeleteV1, dtos.NewSyncTimetableV1, dtos.NewSyncTimetableDeleteV1),
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
			for row, err := range deletesAfter[A](ctx, repo, newestID(cursor[upserts], cursor[deletes]), nowID) {
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

// A client with no ack for a stream holds none of its rows, so it needs no tombstones.
// Tombstones older than its upsert ack were already left out of the upserts it got.
func deletesAfter[A sync.Row](ctx context.Context, repo *repositories.SyncRepository, after, nowID uuid.UUID) iter.Seq2[A, error] {
	if after == uuid.Nil {
		return func(func(A, error) bool) {}
	}
	return repositories.Deletes[A](ctx, repo, after, nowID)
}

func newestID(a, b uuid.UUID) uuid.UUID {
	if bytes.Compare(a[:], b[:]) > 0 {
		return a
	}
	return b
}

type SyncPlan struct {
	protocol SyncProtocol
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

// Only the complete ack is checked because it is minted fresh on every sync. A row ack
// can be old simply because the row has not changed, and would reset the client forever.
// A client holding acks but no complete ack never finished a sync, so it cannot be trusted either.
func (p SyncPlan) needsReset(now time.Time, resetBefore uuid.UUID) bool {
	if len(p.cursor) == 0 {
		return false
	}
	return p.complete == uuid.Nil ||
		sync.Expired(p.complete, now) ||
		bytes.Compare(p.complete[:], resetBefore[:]) < 0
}

type SyncService struct {
	repo *repositories.SyncRepository
}

func NewSyncService(repo *repositories.SyncRepository) *SyncService {
	return &SyncService{repo: repo}
}

// Plan validates a request before any line is written, so every error it returns is the client's.
func (s *SyncService) Plan(req dtos.SyncRequest) (SyncPlan, error) {
	protocol, ok := SyncProtocols[req.Protocol]
	if !ok {
		return SyncPlan{}, fmt.Errorf("%w: unknown protocol %d", ErrInvalidSyncRequest, req.Protocol)
	}
	streams, err := planStreams(req.Types)
	if err != nil {
		return SyncPlan{}, err
	}
	cursor, err := parseAcks(req.Acks)
	if err != nil {
		return SyncPlan{}, err
	}
	return SyncPlan{
		protocol: protocol,
		streams:  streams,
		cursor:   cursor,
		complete: cursor[protocol.Complete],
	}, nil
}

func (s *SyncService) Stream(ctx context.Context, plan SyncPlan, send SyncSender) error {
	nowID, err := s.repo.NowID(ctx)
	if err != nil {
		return err
	}
	resetBefore, err := s.repo.ResetBefore(ctx)
	if err != nil {
		return err
	}

	if plan.needsReset(time.Now(), resetBefore) {
		return send(syncLine(plan.protocol.Reset, nowID, dtos.SyncPayloads[plan.protocol.Reset]))
	}

	for _, stream := range plan.streams {
		if err := stream.run(ctx, s.repo, plan.cursor, nowID, send); err != nil {
			return fmt.Errorf("stream %s: %w", stream.Request, err)
		}
	}

	return send(syncLine(plan.protocol.Complete, nowID, dtos.SyncPayloads[plan.protocol.Complete]))
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

// A client may send several acks for one entity type; the newest wins.
func parseAcks(acks []string) (syncCursor, error) {
	cursor := syncCursor{}
	for _, ack := range acks {
		entity, id, err := ParseAck(ack)
		if err != nil {
			return nil, err
		}
		cursor[entity] = newestID(id, cursor[entity])
	}
	return cursor, nil
}

func ParseAck(ack string) (dtos.SyncEntityType, uuid.UUID, error) {
	rawEntity, rawID, ok := strings.Cut(ack, ackSeparator)
	if !ok {
		return "", uuid.Nil, fmt.Errorf("%w: ack %q has no %q", ErrInvalidSyncRequest, ack, ackSeparator)
	}

	entity := dtos.SyncEntityType(rawEntity)
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
