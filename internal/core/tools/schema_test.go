package tools

import (
	"testing"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
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

// --- Nesting / recursion tests (MP6 Section 3) ---

type schemaInner struct {
	Name string `json:"name"`
}

type schemaOuter struct {
	Inner schemaInner `json:"inner"`
}

type schemaPtrOuter struct {
	Config *schemaInner `json:"config,omitempty"`
}

type schemaNode struct {
	Next *schemaNode `json:"next,omitempty"`
}

type schemaStructSlice struct {
	Items []schemaInner `json:"items"`
}

type schemaPtrStructSlice struct {
	Items []*schemaInner `json:"items"`
}

type schemaScalarSlice struct {
	Tags []string `json:"tags,omitempty"`
}

// 3.1 — nested struct field produces inline object schema with properties
// and required.
func TestSchemaFromStructNestedStruct(t *testing.T) {
	schema, err := SchemaFromStruct(schemaOuter{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	innerProp, ok := props["inner"].(map[string]any)
	if !ok {
		t.Fatalf("inner property is not a map: %T", props["inner"])
	}
	if got := innerProp["type"]; got != "object" {
		t.Errorf("inner type = %v, want object", got)
	}
	innerProps, ok := innerProp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("inner properties missing: %v", innerProp)
	}
	nameProp, ok := innerProps["name"].(map[string]any)
	if !ok {
		t.Fatalf("inner.name property missing: %v", innerProps)
	}
	if got := nameProp["type"]; got != "string" {
		t.Errorf("inner.name type = %v, want string", got)
	}
	innerRequired, ok := innerProp["required"].([]string)
	if !ok || len(innerRequired) != 1 || innerRequired[0] != "name" {
		t.Errorf("inner required = %v, want [name]", innerProp["required"])
	}
	if !requiredContains(t, schema, "inner") {
		t.Error("inner should be required (no omitempty)")
	}
}

// 3.2 — pointer-to-struct field unwrapped to element type, omitempty excludes
// from required.
func TestSchemaFromStructPointerToStructField(t *testing.T) {
	schema, err := SchemaFromStruct(schemaPtrOuter{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	configProp, ok := props["config"].(map[string]any)
	if !ok {
		t.Fatalf("config property is not a map: %T", props["config"])
	}
	if got := configProp["type"]; got != "object" {
		t.Errorf("config type = %v, want object (pointer should be unwrapped)", got)
	}
	configProps, ok := configProp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("config properties missing: %v", configProp)
	}
	if _, ok := configProps["name"]; !ok {
		t.Error("config.name property missing — pointer was not unwrapped to struct")
	}
	if requiredContains(t, schema, "config") {
		t.Error("config should not be required (omitempty)")
	}
}

// 3.3 — self-referential struct (Node{Next *Node}) terminates without panic,
// next property is {}.
func TestSchemaFromStructSelfReferential(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SchemaFromStruct panicked on self-referential struct: %v", r)
		}
	}()

	schema, err := SchemaFromStruct(schemaNode{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	nextProp, ok := props["next"].(map[string]any)
	if !ok {
		t.Fatalf("next property is not a map: %T", props["next"])
	}
	if _, hasType := nextProp["type"]; hasType {
		t.Errorf("next property should be opaque {} (cycle), got %v", nextProp)
	}
	if len(nextProp) != 0 {
		t.Errorf("next property should be empty {} (cycle), got %v", nextProp)
	}
}

// 3.4 — []Struct slice produces {type: "array", items: {type: "object", ...}}
// with correct item schema.
func TestSchemaFromStructStructSlice(t *testing.T) {
	schema, err := SchemaFromStruct(schemaStructSlice{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	itemsProp, ok := props["items"].(map[string]any)
	if !ok {
		t.Fatalf("items property is not a map: %T", props["items"])
	}
	if got := itemsProp["type"]; got != "array" {
		t.Errorf("items type = %v, want array", got)
	}
	itemsSchema, ok := itemsProp["items"].(map[string]any)
	if !ok {
		t.Fatalf("items.items missing: %v", itemsProp)
	}
	if got := itemsSchema["type"]; got != "object" {
		t.Errorf("items.items type = %v, want object", got)
	}
	itemProps, ok := itemsSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("items.items.properties missing: %v", itemsSchema)
	}
	if _, ok := itemProps["name"]; !ok {
		t.Error("items.items.properties.name missing")
	}
}

// 3.4b — []*Struct slice produces array with unwrapped struct item schema.
func TestSchemaFromStructPtrStructSlice(t *testing.T) {
	schema, err := SchemaFromStruct(schemaPtrStructSlice{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	itemsProp, ok := props["items"].(map[string]any)
	if !ok {
		t.Fatalf("items property is not a map: %T", props["items"])
	}
	if got := itemsProp["type"]; got != "array" {
		t.Errorf("items type = %v, want array", got)
	}
	itemsSchema, ok := itemsProp["items"].(map[string]any)
	if !ok {
		t.Fatalf("items.items missing: %v", itemsProp)
	}
	if got := itemsSchema["type"]; got != "object" {
		t.Errorf("items.items type = %v, want object (pointer unwrapped)", got)
	}
}

// 3.5 — []string slice produces {type: "array", items: {type: "string"}}.
func TestSchemaFromStructScalarSlice(t *testing.T) {
	schema, err := SchemaFromStruct(schemaScalarSlice{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	props := schema["properties"].(map[string]any)
	tagsProp, ok := props["tags"].(map[string]any)
	if !ok {
		t.Fatalf("tags property is not a map: %T", props["tags"])
	}
	if got := tagsProp["type"]; got != "array" {
		t.Errorf("tags type = %v, want array", got)
	}
	itemsSchema, ok := tagsProp["items"].(map[string]any)
	if !ok {
		t.Fatalf("tags.items missing: %v", tagsProp)
	}
	if got := itemsSchema["type"]; got != "string" {
		t.Errorf("tags.items type = %v, want string", got)
	}
}

// 3.6 — SchemaFromStruct(&CodePlan{}) produces a complete schema with files as
// an array of objects containing path, types, functions.
func TestSchemaFromStructCodePlan(t *testing.T) {
	schema, err := SchemaFromStruct(&domain.CodePlan{})
	if err != nil {
		t.Fatalf("SchemaFromStruct returned error: %v", err)
	}
	if got := schema["type"]; got != "object" {
		t.Errorf("type = %v, want object", got)
	}
	props := schema["properties"].(map[string]any)

	// files is an array of objects
	filesProp, ok := props["files"].(map[string]any)
	if !ok {
		t.Fatalf("files property is not a map: %T", props["files"])
	}
	if got := filesProp["type"]; got != "array" {
		t.Errorf("files type = %v, want array", got)
	}
	filesItems, ok := filesProp["items"].(map[string]any)
	if !ok {
		t.Fatalf("files.items missing: %v", filesProp)
	}
	if got := filesItems["type"]; got != "object" {
		t.Errorf("files.items type = %v, want object", got)
	}
	fileProps, ok := filesItems["properties"].(map[string]any)
	if !ok {
		t.Fatalf("files.items.properties missing: %v", filesItems)
	}
	for _, key := range []string{"path", "description", "types", "functions"} {
		if _, ok := fileProps[key]; !ok {
			t.Errorf("files.items.properties.%s missing", key)
		}
	}

	// types is an array of objects with name, description, fields
	typesProp, ok := fileProps["types"].(map[string]any)
	if !ok {
		t.Fatalf("types property is not a map: %T", fileProps["types"])
	}
	if got := typesProp["type"]; got != "array" {
		t.Errorf("types type = %v, want array", got)
	}
	typesItems, ok := typesProp["items"].(map[string]any)
	if !ok {
		t.Fatalf("types.items missing: %v", typesProp)
	}
	if got := typesItems["type"]; got != "object" {
		t.Errorf("types.items type = %v, want object", got)
	}
	typeProps, ok := typesItems["properties"].(map[string]any)
	if !ok {
		t.Fatalf("types.items.properties missing: %v", typesItems)
	}
	for _, key := range []string{"name", "description", "fields"} {
		if _, ok := typeProps[key]; !ok {
			t.Errorf("types.items.properties.%s missing", key)
		}
	}

	// functions is an array of objects with name, signature, description
	funcsProp, ok := fileProps["functions"].(map[string]any)
	if !ok {
		t.Fatalf("functions property is not a map: %T", fileProps["functions"])
	}
	if got := funcsProp["type"]; got != "array" {
		t.Errorf("functions type = %v, want array", got)
	}
	funcsItems, ok := funcsProp["items"].(map[string]any)
	if !ok {
		t.Fatalf("functions.items missing: %v", funcsProp)
	}
	if got := funcsItems["type"]; got != "object" {
		t.Errorf("functions.items type = %v, want object", got)
	}
	funcProps, ok := funcsItems["properties"].(map[string]any)
	if !ok {
		t.Fatalf("functions.items.properties missing: %v", funcsItems)
	}
	for _, key := range []string{"name", "signature", "description"} {
		if _, ok := funcProps[key]; !ok {
			t.Errorf("functions.items.properties.%s missing", key)
		}
	}
}
