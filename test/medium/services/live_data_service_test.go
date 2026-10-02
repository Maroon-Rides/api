package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
	requester "github.com/MaroonRides/api/apps/requester/services"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/test"
)

const nextDepartTimesPath = "/RouteMap/GetNextDepartTimes"

var routeDirectionKeyField = regexp.MustCompile(`^routeDirectionKeys\[(\d+)\]\[(routeKey|directionKey)\]$`)

// fakeBusAPI records the route directions asked for at each stop.
type fakeBusAPI struct {
	mu       sync.Mutex
	requests map[string][][]busapi.RouteDirectionPair
}

func newFakeBusAPI() (*fakeBusAPI, *busapi.Client) {
	fake := &fakeBusAPI{requests: map[string][][]busapi.RouteDirectionPair{}}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	DeferCleanup(server.Close)

	client := busapi.NewClient(busapi.ClientConfig{
		Auth:       busapi.StaticAuth{},
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	return fake, client
}

func (f *fakeBusAPI) serve(w http.ResponseWriter, r *http.Request) {
	defer GinkgoRecover()
	Expect(r.URL.Path).To(Equal(nextDepartTimesPath))

	body, err := io.ReadAll(r.Body)
	Expect(err).NotTo(HaveOccurred())
	form, err := url.ParseQuery(string(body))
	Expect(err).NotTo(HaveOccurred())

	f.mu.Lock()
	f.requests[form.Get("stopCode")] = append(f.requests[form.Get("stopCode")], routeDirectionPairs(form))
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"amenities":[],"routeDirectionTimes":[]}`)
}

func routeDirectionPairs(form url.Values) []busapi.RouteDirectionPair {
	byIndex := map[string]*busapi.RouteDirectionPair{}
	for field, values := range form {
		match := routeDirectionKeyField.FindStringSubmatch(field)
		if match == nil {
			continue
		}
		pair, ok := byIndex[match[1]]
		if !ok {
			pair = &busapi.RouteDirectionPair{}
			byIndex[match[1]] = pair
		}
		if match[2] == "routeKey" {
			pair.RouteKey = values[0]
		} else {
			pair.DirectionKey = values[0]
		}
	}

	var pairs []busapi.RouteDirectionPair
	for _, pair := range byIndex {
		pairs = append(pairs, *pair)
	}
	return pairs
}

func pairOf(n test.Network) busapi.RouteDirectionPair {
	return busapi.RouteDirectionPair{RouteKey: n.Route.SourceID, DirectionKey: n.Direction.SourceID}
}

func subscribe(bundb *bun.DB, routeID uuid.UUID) {
	test.Insert(bundb, &model.LiveDataSubscription{RouteID: routeID, ClientID: uuid.New()})
}

var _ = Describe("LiveDataService departures", func() {
	var (
		ctx      = context.Background()
		bundb    *bun.DB
		fake     *fakeBusAPI
		svc      *requester.LiveDataService
		a, b     test.Network
		sharedID string
	)

	BeforeEach(func() {
		bundb = test.Configure()

		var client *busapi.Client
		fake, client = newFakeBusAPI()
		svc = requester.NewLiveDataService(client, repositories.NewRouteDataRepository(bundb))

		a = test.CreateNetwork(bundb, "a")
		b = test.CreateNetwork(bundb, "b")
		test.Insert(bundb, &model.Stop{DirectionID: b.Direction.ID, SourceID: a.Stop.SourceID, Name: a.Stop.Name, Amenities: []string{}, Sequence: 2})
		sharedID = a.Stop.SourceID
	})

	It("asks for every subscribed route at a shared stop in one call", func() {
		subscribe(bundb, a.Route.ID)
		subscribe(bundb, b.Route.ID)

		Expect(svc.SyncSubscribedDepartures(ctx)).To(Succeed())

		Expect(fake.requests).To(HaveLen(2))
		Expect(fake.requests[sharedID]).To(HaveLen(1))
		Expect(fake.requests[sharedID][0]).To(ConsistOf(pairOf(a), pairOf(b)))
		Expect(fake.requests[b.Stop.SourceID]).To(ConsistOf(ConsistOf(pairOf(b))))
	})

	It("leaves unsubscribed routes out of a shared stop", func() {
		subscribe(bundb, a.Route.ID)

		Expect(svc.SyncSubscribedDepartures(ctx)).To(Succeed())

		Expect(fake.requests).To(HaveLen(1))
		Expect(fake.requests[sharedID]).To(ConsistOf(ConsistOf(pairOf(a))))
	})

	It("leaves out routes whose only subscription went stale", func() {
		subscribe(bundb, a.Route.ID)
		subscribe(bundb, b.Route.ID)
		makeStale(bundb, b.Route.ID)

		Expect(svc.SyncSubscribedDepartures(ctx)).To(Succeed())

		Expect(fake.requests).To(HaveLen(1))
		Expect(fake.requests[sharedID]).To(ConsistOf(ConsistOf(pairOf(a))))
	})
})

var _ = Describe("LiveDataService subscription reaper", func() {
	var (
		ctx   = context.Background()
		bundb *bun.DB
		svc   *requester.LiveDataService
		fresh test.Network
		stale test.Network
	)

	BeforeEach(func() {
		bundb = test.Configure()
		_, client := newFakeBusAPI()
		svc = requester.NewLiveDataService(client, repositories.NewRouteDataRepository(bundb))

		fresh = test.CreateNetwork(bundb, "fresh")
		stale = test.CreateNetwork(bundb, "stale")
		subscribe(bundb, fresh.Route.ID)
		subscribe(bundb, stale.Route.ID)
		test.Exec(bundb, `UPDATE "route" SET "liveDataAvailable" = TRUE`)
		makeStale(bundb, stale.Route.ID)
	})

	It("deletes only subscriptions that missed their refreshes", func() {
		Expect(svc.ReapStaleSubscriptions(ctx)).To(Succeed())

		var routeIDs []uuid.UUID
		Expect(bundb.NewSelect().Model((*model.LiveDataSubscription)(nil)).Column("routeId").Scan(ctx, &routeIDs)).To(Succeed())
		Expect(routeIDs).To(Equal([]uuid.UUID{fresh.Route.ID}))
	})

	It("turns live data off for a route left with no subscriptions", func() {
		Expect(svc.ReapStaleSubscriptions(ctx)).To(Succeed())

		Expect(liveDataAvailable(bundb, stale.Route.ID)).To(BeFalse())
		Expect(liveDataAvailable(bundb, fresh.Route.ID)).To(BeTrue())
	})
})

func makeStale(bundb *bun.DB, routeID uuid.UUID) {
	test.Exec(bundb,
		`UPDATE "live_data_subscriptions" SET "lastUpdatedAt" = NOW() - make_interval(secs => ?) WHERE "routeId" = ?`,
		(model.LiveDataSubscriptionStaleAfter + time.Second).Seconds(), routeID)
}

func liveDataAvailable(bundb *bun.DB, routeID uuid.UUID) bool {
	var available bool
	err := bundb.NewSelect().Model((*model.Route)(nil)).Column("liveDataAvailable").
		Where(`"id" = ?`, routeID).Scan(context.Background(), &available)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return available
}
