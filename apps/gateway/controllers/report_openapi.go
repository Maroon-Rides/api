package controllers

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"

	"github.com/MaroonRides/api/apps/gateway/dtos"
)

const (
	reportEventName         = "ReportEvent"
	reportEventSuffix       = "Event"
	reportEventTypeField    = "type"
	reportEventPayloadField = "payload"
)

// fuego cannot describe a union, so ReportEvent is rebuilt as a oneOf with one variant per event type, keyed on `type`.
func registerReportEvent(o *fuego.OpenAPI) {
	schemas := o.Description().Components.Schemas
	union := &openapi3.Schema{
		Discriminator: &openapi3.Discriminator{
			PropertyName: reportEventTypeField,
			Mapping:      map[string]openapi3.MappingRef{},
		},
	}

	for _, value := range dtos.ReportEventType("").EnumValues() {
		eventType := value.(dtos.ReportEventType)
		payload := fuego.SchemaTagFromType(o, dtos.ReportPayloads[eventType])

		name := string(eventType) + reportEventSuffix
		ref := schemaRefPrefix + name
		schemas[name] = openapi3.NewSchemaRef("", reportEventVariant(eventType, payload))

		union.OneOf = append(union.OneOf, openapi3.NewSchemaRef(ref, schemas[name].Value))
		union.Discriminator.Mapping[string(eventType)] = openapi3.MappingRef{Ref: ref}
	}

	schemas[reportEventName] = openapi3.NewSchemaRef("", union)
	fuego.SchemaTagFromType(o, dtos.ReportEventType(""))
}

func reportEventVariant(eventType dtos.ReportEventType, payload fuego.SchemaTag) *openapi3.Schema {
	return &openapi3.Schema{
		Type:     &openapi3.Types{openapi3.TypeObject},
		Required: []string{reportEventTypeField, reportEventPayloadField},
		Properties: openapi3.Schemas{
			reportEventTypeField: openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:  &openapi3.Types{openapi3.TypeString},
				Const: string(eventType),
			}),
			reportEventPayloadField: openapi3.NewSchemaRef(payload.Ref, payload.Value),
		},
	}
}
