package controllers

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"

	"github.com/MaroonRides/api/apps/gateway/dtos"
)

const (
	schemaRefPrefix    = "#/components/schemas/"
	syncStreamLineName = "SyncStreamLine"
	syncLineSuffix     = "Line"
	syncLineTypeField  = "type"
	syncLineAckField   = "ack"
	syncLineDataField  = "data"
)

// fuego cannot describe a union, so SyncStreamLine is rebuilt as a oneOf with one variant per entity type, keyed on `type`.
func registerSyncStreamLine(o *fuego.OpenAPI) {
	schemas := o.Description().Components.Schemas
	union := &openapi3.Schema{
		Discriminator: &openapi3.Discriminator{
			PropertyName: syncLineTypeField,
			Mapping:      map[string]openapi3.MappingRef{},
		},
	}

	for _, value := range dtos.SyncEntityType("").EnumValues() {
		entity := value.(dtos.SyncEntityType)
		payload := fuego.SchemaTagFromType(o, dtos.SyncPayloads[entity])

		name := payload.Name + syncLineSuffix
		ref := schemaRefPrefix + name
		schemas[name] = openapi3.NewSchemaRef("", syncLineVariant(entity, payload))

		union.OneOf = append(union.OneOf, openapi3.NewSchemaRef(ref, schemas[name].Value))
		union.Discriminator.Mapping[string(entity)] = openapi3.MappingRef{Ref: ref}
	}

	schemas[syncStreamLineName] = openapi3.NewSchemaRef("", union)

	// Each variant pins `type` with a const, so nothing else pulls the enum into the spec for clients.
	fuego.SchemaTagFromType(o, dtos.SyncEntityType(""))
}

func syncLineVariant(entity dtos.SyncEntityType, payload fuego.SchemaTag) *openapi3.Schema {
	return &openapi3.Schema{
		Type:     &openapi3.Types{openapi3.TypeObject},
		Required: []string{syncLineTypeField, syncLineAckField, syncLineDataField},
		Properties: openapi3.Schemas{
			syncLineTypeField: openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:  &openapi3.Types{openapi3.TypeString},
				Const: string(entity),
			}),
			syncLineAckField:  openapi3.NewSchemaRef("", openapi3.NewStringSchema()),
			syncLineDataField: openapi3.NewSchemaRef(payload.Ref, payload.Value),
		},
	}
}
