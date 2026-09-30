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
	"github.com/samber/lo"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/sync"
)

var ErrInvalidSyncRequest = errors.New("invalid sync request")

const (
	ackSeparator   = "|"
	scopeSeparator = ":"
)

type SyncSender func(dtos.SyncStreamLine) error

// SyncAckKey names one stream position. A scoped stream keeps one per scope id,
// so a client that adds a scope id starts that id from the beginning.
type SyncAckKey struct {
	Entity dtos.SyncEntityType
	Scope  uuid.UUID
}

func (k SyncAckKey) String() string {
	if k.Scope == uuid.Nil {
		return string(k.Entity)
	}
	return string(k.Entity) + scopeSeparator + k.Scope.String()
}

type syncCursor map[SyncAckKey]uuid.UUID

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

	ScopeType   dtos.SyncScopeType
	ScopeColumn string

	run func(ctx context.Context, repo *repositories.SyncRepository, cursor syncCursor, scope repositories.Scope, nowID uuid.UUID, send SyncSender) error
}

// Parents come before children so a client can apply lines in the order they arrive.
var SyncStreams = []SyncStream{
	NewSyncStream(dtos.SyncRequestTypes.RoutesV1, dtos.SyncEntityTypes.RouteV1, dtos.SyncEntityTypes.RouteDeleteV1, dtos.NewSyncRouteV1, dtos.NewSyncRouteDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.DirectionsV1, dtos.SyncEntityTypes.DirectionV1, dtos.SyncEntityTypes.DirectionDeleteV1, dtos.NewSyncDirectionV1, dtos.NewSyncDirectionDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.StopsV1, dtos.SyncEntityTypes.StopV1, dtos.SyncEntityTypes.StopDeleteV1, dtos.NewSyncStopV1, dtos.NewSyncStopDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.DirectionStopsV1, dtos.SyncEntityTypes.DirectionStopV1, dtos.SyncEntityTypes.DirectionStopDeleteV1, dtos.NewSyncDirectionStopV1, dtos.NewSyncDirectionStopDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.AlertsV1, dtos.SyncEntityTypes.AlertV1, dtos.SyncEntityTypes.AlertDeleteV1, dtos.NewSyncAlertV1, dtos.NewSyncAlertDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.AlertDirectionsV1, dtos.SyncEntityTypes.AlertDirectionV1, dtos.SyncEntityTypes.AlertDirectionDeleteV1, dtos.NewSyncAlertDirectionV1, dtos.NewSyncAlertDirectionDeleteV1),
	NewSyncStream(dtos.SyncRequestTypes.TimetablesV1, dtos.SyncEntityTypes.TimetableV1, dtos.SyncEntityTypes.TimetableDeleteV1, dtos.NewSyncTimetableV1, dtos.NewSyncTimetableDeleteV1).
		ScopedBy(dtos.SyncScopeTypes.OfflineRoutesV1, model.RouteScope),
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
		run: func(ctx context.Context, repo *repositories.SyncRepository, cursor syncCursor, scope repositories.Scope, nowID uuid.UUID, send SyncSender) error {
			upsertKey := SyncAckKey{Entity: upserts, Scope: scope.ID}
			deleteKey := SyncAckKey{Entity: deletes, Scope: scope.ID}

			for row, err := range deletesAfter[A](ctx, repo, scope, newestID(cursor[upsertKey], cursor[deleteKey]), nowID) {
				if err != nil {
					return err
				}
				if err := send(syncLine(deleteKey, row.SyncID(), toDelete(row))); err != nil {
					return err
				}
			}
			for row, err := range repositories.Upserts[M](ctx, repo, scope, cursor[upsertKey], nowID) {
				if err != nil {
					return err
				}
				if err := send(syncLine(upsertKey, row.SyncID(), toUpsert(row))); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// ScopedBy sends the stream only for the ids a request lists under scopeType, matched against column.
func (s SyncStream) ScopedBy(scopeType dtos.SyncScopeType, column string) SyncStream {
	s.ScopeType = scopeType
	s.ScopeColumn = column
	return s
}

// A client with no ack for a stream holds none of its rows, so it needs no tombstones.
// Tombstones older than its upsert ack were already left out of the upserts it got.
func deletesAfter[A sync.Row](
	ctx context.Context,
	repo *repositories.SyncRepository,
	scope repositories.Scope,
	after, nowID uuid.UUID,
) iter.Seq2[A, error] {
	if after == uuid.Nil {
		return func(func(A, error) bool) {}
	}
	return repositories.Deletes[A](ctx, repo, scope, after, nowID)
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
	scopes   map[dtos.SyncScopeType][]uuid.UUID
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

func (p SyncPlan) scopesFor(s SyncStream) []repositories.Scope {
	if s.ScopeType == "" {
		return []repositories.Scope{{}}
	}
	return lo.Map(p.scopes[s.ScopeType], func(id uuid.UUID, _ int) repositories.Scope {
		return repositories.Scope{Column: s.ScopeColumn, ID: id}
	})
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
	scopes, err := parseScopes(req.Scopes)
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
		scopes:   scopes,
		cursor:   cursor,
		complete: cursor[SyncAckKey{Entity: protocol.Complete}],
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
		return send(syncLine(SyncAckKey{Entity: plan.protocol.Reset}, nowID, dtos.SyncPayloads[plan.protocol.Reset]))
	}

	for _, stream := range plan.streams {
		for _, scope := range plan.scopesFor(stream) {
			if err := stream.run(ctx, s.repo, plan.cursor, scope, nowID, send); err != nil {
				return fmt.Errorf("stream %s: %w", stream.Request, err)
			}
		}
	}

	return send(syncLine(SyncAckKey{Entity: plan.protocol.Complete}, nowID, dtos.SyncPayloads[plan.protocol.Complete]))
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

func parseScopes(scopes []dtos.SyncScope) (map[dtos.SyncScopeType][]uuid.UUID, error) {
	parsed := map[dtos.SyncScopeType][]uuid.UUID{}
	for _, scope := range scopes {
		if !slices.Contains(dtos.SyncScopeType("").EnumValues(), any(scope.Type)) {
			return nil, fmt.Errorf("%w: unknown scope type %q", ErrInvalidSyncRequest, scope.Type)
		}
		if _, ok := parsed[scope.Type]; ok {
			return nil, fmt.Errorf("%w: scope type %q is listed twice", ErrInvalidSyncRequest, scope.Type)
		}
		if slices.Contains(scope.IDs, uuid.Nil) {
			return nil, fmt.Errorf("%w: scope type %q lists a nil id", ErrInvalidSyncRequest, scope.Type)
		}
		parsed[scope.Type] = lo.Uniq(scope.IDs)
	}
	return parsed, nil
}

// A client may send several acks for one key; the newest wins.
func parseAcks(acks []string) (syncCursor, error) {
	cursor := syncCursor{}
	for _, ack := range acks {
		key, id, err := ParseAck(ack)
		if err != nil {
			return nil, err
		}
		cursor[key] = newestID(id, cursor[key])
	}
	return cursor, nil
}

func ParseAck(ack string) (SyncAckKey, uuid.UUID, error) {
	rawKey, rawID, ok := strings.Cut(ack, ackSeparator)
	if !ok {
		return SyncAckKey{}, uuid.Nil, fmt.Errorf("%w: ack %q has no %q", ErrInvalidSyncRequest, ack, ackSeparator)
	}

	rawEntity, rawScope, scoped := strings.Cut(rawKey, scopeSeparator)
	entity := dtos.SyncEntityType(rawEntity)
	if !slices.Contains(dtos.SyncEntityType("").EnumValues(), any(entity)) {
		return SyncAckKey{}, uuid.Nil, fmt.Errorf("%w: ack %q has unknown type", ErrInvalidSyncRequest, ack)
	}

	key := SyncAckKey{Entity: entity}
	switch wantsScope := entityScope(entity) != ""; {
	case scoped && !wantsScope:
		return SyncAckKey{}, uuid.Nil, fmt.Errorf("%w: ack %q is scoped but %s is not", ErrInvalidSyncRequest, ack, entity)
	case !scoped && wantsScope:
		return SyncAckKey{}, uuid.Nil, fmt.Errorf("%w: ack %q needs a scope id", ErrInvalidSyncRequest, ack)
	case scoped:
		scope, err := uuid.Parse(rawScope)
		if err != nil || scope == uuid.Nil {
			return SyncAckKey{}, uuid.Nil, fmt.Errorf("%w: ack %q has a bad scope id", ErrInvalidSyncRequest, ack)
		}
		key.Scope = scope
	}

	id, err := uuid.Parse(rawID)
	if _, minted := sync.MintedAt(id); err != nil || !minted {
		return SyncAckKey{}, uuid.Nil, fmt.Errorf("%w: ack %q does not end in a uuidv7", ErrInvalidSyncRequest, ack)
	}
	return key, id, nil
}

func entityScope(entity dtos.SyncEntityType) dtos.SyncScopeType {
	stream, _ := lo.Find(SyncStreams, func(s SyncStream) bool { return s.Upserts == entity || s.Deletes == entity })
	return stream.ScopeType
}

func FormatAck(key SyncAckKey, id uuid.UUID) string {
	return key.String() + ackSeparator + id.String()
}

func syncLine(key SyncAckKey, id uuid.UUID, data any) dtos.SyncStreamLine {
	return dtos.SyncStreamLine{Type: key.Entity, Ack: FormatAck(key, id), Data: data}
}
