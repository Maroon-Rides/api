package main

import (
	"reflect"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
)

type enumValuer interface {
	EnumValues() []any
}

// Title carries the Go type name so promoteEnum can hoist the enum.
func schemaCustomizer(_ string, t reflect.Type, tag reflect.StructTag, schema *openapi3.Schema) error {
	if t == reflect.TypeFor[uuid.UUID]() {
		nullable := schema.Nullable
		*schema = *openapi3.NewUUIDSchema()
		schema.Nullable = nullable
	}
	if strings.Contains(tag.Get("json"), "omitempty") {
		schema.Nullable = false
	}
	if v, ok := reflect.New(t).Interface().(enumValuer); ok {
		schema.Enum = v.EnumValues()
		schema.Title = t.Name()
	}
	if format := tag.Get("format"); format != "" {
		schema.Format = format
	}
	return nil
}

func sanitizeOpenAPISpec(doc *openapi3.T) {
	walkSchemas(doc, func(ref *openapi3.SchemaRef) bool {
		return !promoteEnum(doc, ref)
	})
}

// A visitor returning false stops descent; $ref nodes are skipped because their target is reached through doc.Components.
func walkSchemas(doc *openapi3.T, visit func(*openapi3.SchemaRef) bool) {
	if doc == nil || doc.Components == nil {
		return
	}

	var walk func(*openapi3.SchemaRef)
	walk = func(ref *openapi3.SchemaRef) {
		if ref == nil || ref.Ref != "" || ref.Value == nil {
			return
		}
		if !visit(ref) {
			return
		}
		for _, p := range ref.Value.Properties {
			walk(p)
		}
		walk(ref.Value.Items)
		walk(ref.Value.AdditionalProperties.Schema)
	}

	for _, ref := range schemaRoots(doc) {
		walk(ref)
	}
}

// Reports whether the node became a $ref, so the walk knows not to descend into the shared component again.
func promoteEnum(doc *openapi3.T, ref *openapi3.SchemaRef) bool {
	s := ref.Value
	if s.Title == "" || len(s.Enum) == 0 || doc.Components.Schemas[s.Title] == ref {
		return false
	}

	if _, ok := doc.Components.Schemas[s.Title]; !ok {
		kept := withoutNull(s.Type)
		doc.Components.Schemas[s.Title] = openapi3.NewSchemaRef("",
			&openapi3.Schema{Type: &kept, Enum: s.Enum})
	}
	ref.Ref = "#/components/schemas/" + s.Title
	ref.Value = doc.Components.Schemas[s.Title].Value
	return true
}

func withoutNull(types *openapi3.Types) openapi3.Types {
	if types == nil {
		return openapi3.Types{}
	}
	kept := make(openapi3.Types, 0, len(*types))
	for _, t := range *types {
		if t != "null" {
			kept = append(kept, t)
		}
	}
	return kept
}

// Components plus every operation's parameter and body schemas: recursing from these reaches every inline schema.
func schemaRoots(doc *openapi3.T) []*openapi3.SchemaRef {
	roots := make([]*openapi3.SchemaRef, 0, len(doc.Components.Schemas))
	for _, ref := range doc.Components.Schemas {
		roots = append(roots, ref)
	}
	if doc.Paths == nil {
		return roots
	}

	addContent := func(c openapi3.Content) {
		for _, mt := range c {
			if mt != nil {
				roots = append(roots, mt.Schema)
			}
		}
	}
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			for _, p := range op.Parameters {
				if p.Value != nil {
					roots = append(roots, p.Value.Schema)
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				addContent(op.RequestBody.Value.Content)
			}
			if op.Responses != nil {
				for _, resp := range op.Responses.Map() {
					if resp.Value != nil {
						addContent(resp.Value.Content)
					}
				}
			}
		}
	}
	return roots
}
