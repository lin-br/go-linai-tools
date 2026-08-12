package tools

import (
	"fmt"
	"reflect"
	"strings"
)

// ToolSchema bundles a tool name, human-readable description, and JSON Schema
// describing the desired structured output. InputSchema is a plain
// map[string]any — the same type as domain.Tool.InputSchema — so it can be
// produced by SchemaFromStruct or constructed by hand for complex schemas.
type ToolSchema struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// SchemaFromStruct generates a basic JSON Schema (type "object" with
// "properties" and "required") from the exported fields of v using reflect.
//
// Field names are derived from json struct tags (the first comma-separated
// part); fields without a json tag use the Go field name. Fields tagged with
// json:"-" are skipped. Fields whose tag includes "omitempty" are excluded
// from the "required" array; all other fields are included.
//
// Go scalar types map to JSON Schema types as follows:
//   - string → "string"
//   - int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64 → "number"
//   - float32, float64 → "number"
//   - bool → "boolean"
//
// Slices and arrays map to {"type": "array"}. All other types (nested structs,
// maps, interfaces) produce an opaque {} — callers with complex schemas should
// build the InputSchema map by hand.
//
// Non-struct input (including pointers to non-structs) returns an error.
func SchemaFromStruct(v any) (map[string]any, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, fmt.Errorf("schema: expected struct, got nil")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("schema: expected struct, got %s", t.Kind())
	}

	properties := make(map[string]any)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		name, omitempty, skip := parseJSONTag(f)
		if skip {
			continue
		}

		properties[name] = schemaForType(f.Type)
		if !omitempty {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema, nil
}

// parseJSONTag extracts the JSON field name, whether omitempty is set, and
// whether the field should be skipped (json:"-"). When no json tag is present
// or the name portion is empty, the Go field name is used.
func parseJSONTag(f reflect.StructField) (name string, omitempty bool, skip bool) {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name, false, false
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return "", false, true
	}
	if parts[0] == "" {
		name = f.Name
	} else {
		name = parts[0]
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty, false
}

// schemaForType returns a JSON Schema fragment for the given Go type. Scalars
// map to {"type": "..."}; slices and arrays map to {"type": "array"}; all
// other types (nested structs, maps, interfaces) map to an opaque {}.
func schemaForType(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array"}
	default:
		return map[string]any{}
	}
}
