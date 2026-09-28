package sync_test

import (
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/internal/db/sync"
)

func uuidV7At(at time.Time) uuid.UUID {
	id := uuid.Must(uuid.NewV7())
	ms := at.UnixMilli()
	for i := 5; i >= 0; i-- {
		id[i] = byte(ms)
		ms >>= 8
	}
	return id
}

var _ = Describe("MintedAt", Label("unit"), func() {
	It("reads the uuidv7 timestamp", func() {
		at := time.Date(2026, 9, 27, 12, 30, 15, int(250*time.Millisecond), time.UTC)

		minted, ok := sync.MintedAt(uuidV7At(at))
		Expect(ok).To(BeTrue())
		Expect(minted).To(BeTemporally("==", at))
	})

	It("rejects other uuid versions", func() {
		_, ok := sync.MintedAt(uuid.New())
		Expect(ok).To(BeFalse())

		_, ok = sync.MintedAt(uuid.Nil)
		Expect(ok).To(BeFalse())
	})
})

var _ = Describe("Expired", Label("unit"), func() {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

	DescribeTable("resets clients whose last complete sync is outside the window",
		func(id uuid.UUID, expired bool) {
			Expect(sync.Expired(id, now)).To(Equal(expired))
		},
		Entry("just now", uuidV7At(now), false),
		Entry("a minute inside the window", uuidV7At(now.Add(-sync.ResetAfter+time.Minute)), false),
		Entry("a minute past the window", uuidV7At(now.Add(-sync.ResetAfter-time.Minute)), true),
		Entry("not a uuidv7", uuid.New(), true),
	)

	It("keeps tombstones longer than the reset window", func() {
		Expect(sync.TombstoneRetention).To(BeNumerically(">", sync.ResetAfter))
	})
})
