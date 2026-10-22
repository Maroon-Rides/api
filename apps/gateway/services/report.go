package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/metrics"
)

var ErrInvalidReport = errors.New("invalid report")

const (
	maxReportEvents = 50
	// A client holds one ack per entity type, so anything past this is not a real client.
	maxReportedAcks    = 64
	maxReportedMessage = 512
)

type syncFailure struct {
	code     metrics.SyncFailureCode
	protocol dtos.SyncProtocolVersion
	acks     []string
	status   int
	message  string
}

type syncFailureDecoder func(json.RawMessage) (syncFailure, error)

var syncFailureDecoders = map[dtos.ReportEventType]syncFailureDecoder{
	dtos.ReportEventTypes.SyncFailedHTTPStatusV1: decodeSyncFailure(func(p dtos.SyncFailedHTTPStatusV1) syncFailure {
		return syncFailure{code: metrics.SyncFailureHTTPStatus, protocol: p.Protocol, acks: p.Acks, status: p.Status, message: p.Message}
	}),
	dtos.ReportEventTypes.SyncFailedInvalidLineV1: decodeSyncFailure(func(p dtos.SyncFailedInvalidLineV1) syncFailure {
		return syncFailure{code: metrics.SyncFailureInvalidLine, protocol: p.Protocol, acks: p.Acks, message: p.Message}
	}),
	dtos.ReportEventTypes.SyncFailedApplyV1: decodeSyncFailure(func(p dtos.SyncFailedApplyV1) syncFailure {
		return syncFailure{code: metrics.SyncFailureApply, protocol: p.Protocol, acks: p.Acks, message: p.Message}
	}),
}

func decodeSyncFailure[P any](toFailure func(P) syncFailure) syncFailureDecoder {
	return func(raw json.RawMessage) (syncFailure, error) {
		var payload P
		if err := json.Unmarshal(raw, &payload); err != nil {
			return syncFailure{}, err
		}
		failure := toFailure(payload)
		if _, ok := SyncProtocols[failure.protocol]; !ok {
			return syncFailure{}, fmt.Errorf("unknown protocol %d", failure.protocol)
		}
		if len(failure.acks) > maxReportedAcks {
			return syncFailure{}, fmt.Errorf("%d acks is more than %d", len(failure.acks), maxReportedAcks)
		}
		return failure, nil
	}
}

type ReportService struct {
	syncMetrics   *metrics.SyncMetrics
	reportMetrics *metrics.ReportMetrics
}

func NewReportService(syncMetrics *metrics.SyncMetrics, reportMetrics *metrics.ReportMetrics) *ReportService {
	return &ReportService{syncMetrics: syncMetrics, reportMetrics: reportMetrics}
}

// Record drops a bad event instead of failing the request, so one event a gateway cannot read never loses the rest.
func (s *ReportService) Record(req dtos.ReportRequest, userAgent string) error {
	if len(req.Events) > maxReportEvents {
		return fmt.Errorf("%w: %d events is more than %d", ErrInvalidReport, len(req.Events), maxReportEvents)
	}
	for _, event := range req.Events {
		s.record(event, truncate(userAgent, maxReportedMessage))
	}
	return nil
}

func (s *ReportService) record(event dtos.ReportEvent, userAgent string) {
	decode, ok := syncFailureDecoders[event.Type]
	if !ok {
		slog.Warn("dropped report event with unknown type", "type", truncate(string(event.Type), maxReportedMessage), "userAgent", userAgent)
		s.reportMetrics.Received(metrics.UnknownReportType, metrics.ReportDropped)
		return
	}

	failure, err := decode(event.Payload)
	if err != nil {
		slog.Warn("dropped report event with bad payload", "type", event.Type, "error", err, "userAgent", userAgent)
		s.reportMetrics.Received(string(event.Type), metrics.ReportDropped)
		return
	}

	// Acks are not parsed, since a sync rejected for a bad ack is exactly what needs reporting.
	start := metrics.SyncStartCheckpoint
	if len(failure.acks) == 0 {
		start = metrics.SyncStartEmpty
	}
	s.syncMetrics.Failed(failure.code, start)
	s.reportMetrics.Received(string(event.Type), metrics.ReportRecorded)

	slog.Warn("client sync failed",
		"type", event.Type,
		"start", start,
		"status", failure.status,
		"message", truncate(failure.message, maxReportedMessage),
		"acks", failure.acks,
		"userAgent", userAgent,
	)
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}
