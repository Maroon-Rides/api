package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/apps/gateway/controllers"
	"github.com/MaroonRides/api/apps/gateway/dtos"
)

const componentPrefix = "#/components/schemas/"

var _ = Describe("OpenAPI spec", Label("unit"), func() {
	var doc *openapi3.T

	component := func(name string) *openapi3.Schema {
		ref, ok := doc.Components.Schemas[name]
		Expect(ok).To(BeTrue(), "no %s component", name)
		return ref.Value
	}

	BeforeEach(func() {
		server := fuego.NewServer()
		registerAPI(server, []controllers.Controller{controllers.NewSyncController(nil)})

		raw, err := json.Marshal(server.OpenAPI.Description())
		Expect(err).NotTo(HaveOccurred())
		doc, err = openapi3.NewLoader().LoadFromData(raw)
		Expect(err).NotTo(HaveOccurred())
	})

	It("validates", func() {
		Expect(doc.Validate(context.Background())).To(Succeed())
	})

	DescribeTable("publishes sync enums as named components",
		func(name string, values []any) {
			var want []any
			for _, v := range values {
				want = append(want, reflectString(v))
			}
			Expect(component(name).Enum).To(ConsistOf(want...))
			Expect(doc.Components.Schemas[name].Ref).To(BeEmpty(), "the component points at itself")
		},
		Entry("request types", "SyncRequestType", dtos.SyncRequestType("").EnumValues()),
		Entry("entity types", "SyncEntityType", dtos.SyncEntityType("").EnumValues()),
	)

	It("refers to the request type enum from the request body", func() {
		types := component("SyncRequest").Properties["types"]
		Expect(types.Value.Items.Ref).To(Equal(componentPrefix + "SyncRequestType"))
	})

	It("describes a stream line as a union keyed on type", func() {
		line := component("SyncStreamLine")
		entities := dtos.SyncEntityType("").EnumValues()

		Expect(line.OneOf).To(HaveLen(len(entities)))
		Expect(line.Discriminator).NotTo(BeNil())
		Expect(line.Discriminator.PropertyName).To(Equal("type"))

		for _, value := range entities {
			entity := string(value.(dtos.SyncEntityType))
			Expect(line.Discriminator.Mapping).To(HaveKey(entity))

			variant := component(strings.TrimPrefix(line.Discriminator.Mapping[entity].Ref, componentPrefix))
			Expect(variant.Properties["type"].Value.Const).To(Equal(entity))
			Expect(variant.Required).To(ConsistOf("type", "ack", "data"))
			Expect(variant.Properties["data"].Ref).To(HavePrefix(componentPrefix))
		}
	})

	It("names each line variant after its payload", func() {
		mapping := component("SyncStreamLine").Discriminator.Mapping

		Expect(mapping["RouteV1"].Ref).To(Equal(componentPrefix + "SyncRouteV1Line"))
		Expect(component("SyncRouteV1Line").Properties["data"].Ref).To(Equal(componentPrefix + "SyncRouteV1"))
	})

	It("serves the stream as JSON lines of SyncStreamLine", func() {
		content := doc.Paths.Find("/api/sync/stream").Post.Responses.Status(200).Value.Content

		Expect(content).To(HaveKey(controllers.JSONLinesContentType))
		Expect(content[controllers.JSONLinesContentType].Schema.Ref).To(Equal(componentPrefix + "SyncStreamLine"))
	})

	It("publishes ids as uuid strings and dates as dates", func() {
		id := component("SyncRouteV1").Properties["id"].Value
		Expect(id.Type.Is(openapi3.TypeString)).To(BeTrue())
		Expect(id.Format).To(Equal("uuid"))

		date := component("SyncStopScheduleV1").Properties["serviceDate"].Value
		Expect(date.Type.Is(openapi3.TypeString)).To(BeTrue())
		Expect(date.Format).To(Equal("date"))
	})
})

func reflectString(v any) any {
	raw, err := json.Marshal(v)
	Expect(err).NotTo(HaveOccurred())
	var s string
	Expect(json.Unmarshal(raw, &s)).To(Succeed())
	return s
}
