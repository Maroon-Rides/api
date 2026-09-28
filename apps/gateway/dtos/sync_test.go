package dtos_test

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/internal/db/model"
)

var _ = Describe("SyncPayloads", Label("unit"), func() {
	It("has exactly one payload per entity type", func() {
		entities := dtos.SyncEntityType("").EnumValues()

		for _, value := range entities {
			Expect(dtos.SyncPayloads).To(HaveKey(value.(dtos.SyncEntityType)))
		}
		Expect(dtos.SyncPayloads).To(HaveLen(len(entities)))
	})
})

var _ = Describe("sync payloads", Label("unit"), func() {
	It("sends a stop schedule's service date as a calendar date", func() {
		payload := dtos.NewSyncStopScheduleV1(model.StopSchedule{
			ServiceDate: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		})

		Expect(payload.ServiceDate).To(Equal("2026-09-27"))
	})

	It("sends an open-ended alert's end as null", func() {
		raw, err := json.Marshal(dtos.NewSyncAlertV1(model.Alert{ID: uuid.Must(uuid.NewV7())}))
		Expect(err).NotTo(HaveOccurred())

		var fields map[string]any
		Expect(json.Unmarshal(raw, &fields)).To(Succeed())
		Expect(fields).To(HaveKeyWithValue("endsAt", BeNil()))
	})
})
