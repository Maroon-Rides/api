package enum

import "reflect"

// Reflects a namespace struct so each enum lists its values exactly once and the OpenAPI generator needs no second hand-maintained list.
func Values(namespace any) []any {
	v := reflect.ValueOf(namespace)
	out := make([]any, 0, v.NumField())
	for _, field := range v.Fields() {
		out = append(out, field.Interface())
	}
	return out
}
