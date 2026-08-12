package tools

import (
	"testing"
)

type schemaPerson struct {
	Name string `json:"name"`
	Age  int    `json:"age,omitempty"`
}

type schemaNoTag struct {
	Title string
}

type schemaUnexported struct {
	Name string `json:"name"`
	age  int
}

type schemaSkipTag struct {
	Name string `json:"name"`
	Pass string `json:"-"`
}

type schemaAllTypes struct {
	Str  string  `json:"str"`
	I    int     `json:"i"`
	I64  int64   `json:"i64"`
	I32  int32   `json:"i32"`
	F64  float64 `json:"f64"`
	F32  float32 `json:"f32"`
	B    bool    `json:"b"`
	UI   uint    `json:"ui"`
	Sli  []string `json:"sli"`
}

type schemaAllOptional struct {
	Name string `json:"name,omitempty"`
	Age  int    `json:"age,omitempty"`
}

func TestSchemaFromStructErrors(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"string", "hello"},
		{"int", 42},
		{"nil", nil},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]int{"a": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SchemaFromStruct(tt.input)
			if err == nil {
				t.Fatalf("SchemaFromStruct(%v) expected error, got nil", tt.input)
			}
		})
	}
}

func TestSchemaFromStructRequiredAndOptional(t *testing.T) {
	schema, err := SchemaFromStruct(schemaPerson{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	if got := schema["type"]; got != "object" {
		t.Errorf("type = %v, want object", got)
	}
	if got := propType(t, schema, "name"); got != "string" {
		t.Errorf("name type = %q, want string", got)
	}
	if got := propType(t, schema, "age"); got != "number" {
		t.Errorf("age type = %q, want number", got)
	}
	if !requiredContains(t, schema, "name") {
		t.Error("name should be required")
	}
	if requiredContains(t, schema, "age") {
		t.Error("age should not be required (omitempty)")
	}
}

func TestSchemaFromStructNoTagUsesGoName(t *testing.T) {
	schema, err := SchemaFromStruct(schemaNoTag{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["Title"]; !ok {
		t.Errorf("expected property %q, got %v", "Title", props)
	}
	if !requiredContains(t, schema, "Title") {
		t.Error("Title should be required (no tag)")
	}
}

func TestSchemaFromStructUnexportedSkipped(t *testing.T) {
	schema, err := SchemaFromStruct(schemaUnexported{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["age"]; ok {
		t.Error("unexported field age should be skipped")
	}
	if _, ok := props["name"]; !ok {
		t.Error("exported field name should be present")
	}
	if !requiredContains(t, schema, "name") {
		t.Error("name should be required")
	}
}

func TestSchemaFromStructSkipTag(t *testing.T) {
	schema, err := SchemaFromStruct(schemaSkipTag{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["Pass"]; ok {
		t.Error(`field with json:"-" should be skipped`)
	}
	if _, ok := props["name"]; !ok {
		t.Error("name should be present")
	}
}

func TestSchemaFromStructTypeMapping(t *testing.T) {
	schema, err := SchemaFromStruct(schemaAllTypes{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	typeCases := []struct {
		field string
		want  string
	}{
		{"str", "string"},
		{"i", "number"},
		{"i64", "number"},
		{"i32", "number"},
		{"f64", "number"},
		{"f32", "number"},
		{"b", "boolean"},
		{"ui", "number"},
	}
	for _, tc := range typeCases {
		t.Run(tc.field, func(t *testing.T) {
			if got := propType(t, schema, tc.field); got != tc.want {
				t.Errorf("property %q type = %q, want %q", tc.field, got, tc.want)
			}
		})
	}
	if got := propType(t, schema, "sli"); got != "array" {
		t.Errorf("property sli type = %q, want array", got)
	}
}

func TestSchemaFromStructPointerToStruct(t *testing.T) {
	schema, err := SchemaFromStruct(&schemaPerson{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	if got := schema["type"]; got != "object" {
		t.Errorf("type = %v, want object", got)
	}
	if got := propType(t, schema, "name"); got != "string" {
		t.Errorf("name type = %q, want string", got)
	}
}

func TestSchemaFromStructAllOptionalOmitsRequired(t *testing.T) {
	schema, err := SchemaFromStruct(schemaAllOptional{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	if _, ok := schema["required"]; ok {
		t.Errorf("required should be omitted when all fields are optional, got %v", schema["required"])
	}
}

func propType(t *testing.T, schema map[string]any, field string) string {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is not a map: %T", schema["properties"])
	}
	prop, ok := props[field].(map[string]any)
	if !ok {
		t.Fatalf("property %q is not a map: %T", field, props[field])
	}
	typ, _ := prop["type"].(string)
	return typ
}

func requiredContains(t *testing.T, schema map[string]any, field string) bool {
	t.Helper()
	required, ok := schema["required"].([]string)
	if !ok {
		return false
	}
	for _, r := range required {
		if r == field {
			return true
		}
	}
	return false
}
