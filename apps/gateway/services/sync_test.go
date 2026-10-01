package services_test

import (
	"reflect"
	"slices"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/internal/db/model"
)

var (
	requests = dtos.SyncRequestTypes
	entities = dtos.SyncEntityTypes
)

func newV7() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

var _ = Describe("SyncStreams", Label("unit"), func() {
	It("has a stream for every request type", func() {
		for _, value := range dtos.SyncRequestType("").EnumValues() {
			request := value.(dtos.SyncRequestType)
			Expect(slices.ContainsFunc(services.SyncStreams, func(s services.SyncStream) bool {
				return s.Request == request
			})).To(BeTrue(), "%s has no stream", request)
		}
	})

	// A new version of a resource mints its own upsert and delete types, so it
	// never reads an older version's acks and always starts from the beginning.
	It("gives every stream its own entity types", func() {
		owner := map[dtos.SyncEntityType]dtos.SyncRequestType{}
		for _, s := range services.SyncStreams {
			for _, entity := range []dtos.SyncEntityType{s.Upserts, s.Deletes} {
				Expect(owner).NotTo(HaveKey(entity), "%s and %s both produce %s", owner[entity], s.Request, entity)
				owner[entity] = s.Request
			}
		}
	})

	It("produces every data entity type", func() {
		var control []dtos.SyncEntityType
		for _, p := range services.SyncProtocols {
			control = append(control, p.Reset, p.Complete)
		}

		for _, value := range dtos.SyncEntityType("").EnumValues() {
			entity := value.(dtos.SyncEntityType)
			if slices.Contains(control, entity) {
				continue
			}
			Expect(slices.ContainsFunc(services.SyncStreams, func(s services.SyncStream) bool {
				return s.Upserts == entity || s.Deletes == entity
			})).To(BeTrue(), "no stream produces %s", entity)
		}
	})

	It("follows the parent-before-child order of model.SyncTables", func() {
		position := map[reflect.Type]int{}
		for i, table := range model.SyncTables {
			position[reflect.TypeOf(table.Model).Elem()] = i
		}

		var order []int
		for _, s := range services.SyncStreams {
			Expect(position).To(HaveKey(s.Model), "%s reads a model outside model.SyncTables", s.Request)
			order = append(order, position[s.Model])
		}
		Expect(slices.IsSorted(order)).To(BeTrue())
	})
})

var _ = Describe("SyncProtocols", Label("unit"), func() {
	It("defines every protocol version", func() {
		for _, value := range dtos.SyncProtocolVersion(0).EnumValues() {
			Expect(services.SyncProtocols).To(HaveKey(value.(dtos.SyncProtocolVersion)))
		}
		Expect(services.SyncProtocols).To(HaveLen(len(dtos.SyncProtocolVersion(0).EnumValues())))
	})

	It("uses control types no stream produces", func() {
		for version, p := range services.SyncProtocols {
			for _, control := range []dtos.SyncEntityType{p.Reset, p.Complete} {
				Expect(slices.ContainsFunc(services.SyncStreams, func(s services.SyncStream) bool {
					return s.Upserts == control || s.Deletes == control
				})).To(BeFalse(), "protocol %d sends %s, which a stream also produces", version, control)
			}
		}
	})

	It("keeps V1 on the V1 control lines", func() {
		Expect(services.SyncProtocols[dtos.SyncProtocolVersions.V1]).To(Equal(services.SyncProtocol{
			Reset: entities.SyncResetV1, Complete: entities.SyncCompleteV1,
		}))
	})
})

var _ = Describe("SyncService.Plan", Label("unit"), func() {
	svc := services.NewSyncService(nil)

	It("orders streams by the server, not the request", func() {
		plan, err := svc.Plan(dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.TimetablesV1, requests.RoutesV1, requests.StopsV1},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.Requests()).To(Equal([]dtos.SyncRequestType{requests.RoutesV1, requests.StopsV1, requests.TimetablesV1}))
	})

	It("ignores a repeated type", func() {
		plan, err := svc.Plan(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: []dtos.SyncRequestType{requests.RoutesV1, requests.RoutesV1}})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.Requests()).To(Equal([]dtos.SyncRequestType{requests.RoutesV1}))
	})

	It("accepts acks for types outside the request", func() {
		_, err := svc.Plan(dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.RoutesV1},
			Acks:     []string{services.FormatAck(entities.StopV1, newV7())},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	When("a second version of a resource exists", func() {
		BeforeEach(func() {
			original := services.SyncStreams
			routesV2 := services.NewSyncStream("RoutesV2", "RouteV2", "RouteDeleteV2", dtos.NewSyncRouteV1, dtos.NewSyncRouteDeleteV1)
			services.SyncStreams = append(slices.Clone(original), routesV2)
			DeferCleanup(func() { services.SyncStreams = original })
		})

		It("rejects both versions in one request", func() {
			_, err := svc.Plan(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: []dtos.SyncRequestType{requests.RoutesV1, "RoutesV2"}})
			Expect(err).To(MatchError(services.ErrInvalidSyncRequest))
		})

		It("accepts either version alone", func() {
			_, err := svc.Plan(dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: []dtos.SyncRequestType{"RoutesV2"}})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	DescribeTable("rejects bad requests",
		func(req dtos.SyncRequest) {
			_, err := svc.Plan(req)
			Expect(err).To(MatchError(services.ErrInvalidSyncRequest))
		},
		Entry("no types", dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1}),
		Entry("no protocol", dtos.SyncRequest{Types: []dtos.SyncRequestType{requests.RoutesV1}}),
		Entry("an unknown protocol", dtos.SyncRequest{Protocol: 99, Types: []dtos.SyncRequestType{requests.RoutesV1}}),
		Entry("an unknown type", dtos.SyncRequest{Protocol: dtos.SyncProtocolVersions.V1, Types: []dtos.SyncRequestType{"TripsV1"}}),
		Entry("an ack without a separator", dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.RoutesV1},
			Acks:     []string{"RouteV1" + newV7().String()},
		}),
		Entry("an ack with an unknown type", dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.RoutesV1},
			Acks:     []string{"TripV1|" + newV7().String()},
		}),
		Entry("an ack naming a request type instead of an entity type", dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.RoutesV1},
			Acks:     []string{"RoutesV1|" + newV7().String()},
		}),
		Entry("an ack with a garbage id", dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.RoutesV1},
			Acks:     []string{"RouteV1|not-a-uuid"},
		}),
		Entry("an ack with a uuidv4", dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.RoutesV1},
			Acks:     []string{"RouteV1|" + uuid.New().String()},
		}),
		Entry("an ack with a trailing field", dtos.SyncRequest{
			Protocol: dtos.SyncProtocolVersions.V1,
			Types:    []dtos.SyncRequestType{requests.RoutesV1},
			Acks:     []string{"RouteV1|" + newV7().String() + "|extra"},
		}),
	)
})

var _ = Describe("acks", Label("unit"), func() {
	It("round-trips through FormatAck and ParseAck", func() {
		id := newV7()

		ack := services.FormatAck(entities.DirectionStopDeleteV1, id)
		Expect(ack).To(Equal("DirectionStopDeleteV1|" + id.String()))

		entity, parsed, err := services.ParseAck(ack)
		Expect(err).NotTo(HaveOccurred())
		Expect(entity).To(Equal(entities.DirectionStopDeleteV1))
		Expect(parsed).To(Equal(id))
	})

	// Clients from before scopes were removed kept one timetable ack per route.
	It("rejects an ack carrying a route", func() {
		_, _, err := services.ParseAck("TimetableV1:" + uuid.New().String() + "|" + newV7().String())
		Expect(err).To(MatchError(services.ErrInvalidSyncRequest))
	})
})
