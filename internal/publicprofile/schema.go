package publicprofile

import (
	"bytes"
	_ "embed"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaResource = "https://wheretoken.local/public-profile.schema.json"

// publicProfileSchema is the runtime contract. A contract test requires the
// published copy in docs/public-profile.schema.json to remain byte-identical.
//
//go:embed public-profile.schema.json
var publicProfileSchema []byte

var (
	compileSchemaOnce sync.Once
	compiledSchema    *jsonschema.Schema
	compileSchemaErr  error
)

func validateSchemaJSON(raw []byte) error {
	sch, err := runtimeSchema()
	if err != nil {
		return fmt.Errorf("publicprofile: compile schema: %w", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("publicprofile: decode JSON: %w", err)
	}
	if err := sch.Validate(doc); err != nil {
		return fmt.Errorf("publicprofile: schema validation: %w", err)
	}
	return nil
}

func runtimeSchema() (*jsonschema.Schema, error) {
	compileSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(publicProfileSchema))
		if err != nil {
			compileSchemaErr = err
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		if err := compiler.AddResource(schemaResource, doc); err != nil {
			compileSchemaErr = err
			return
		}
		compiledSchema, compileSchemaErr = compiler.Compile(schemaResource)
	})
	return compiledSchema, compileSchemaErr
}
