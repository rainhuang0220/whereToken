package publicprofile

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	schemaResourceV2 = "https://wheretoken.local/public-profile.schema.json"
	schemaResourceV1 = "https://wheretoken.local/public-profile.schema.v1.json"
)

// publicProfileSchema is the current (v2) runtime contract. A contract test
// requires the published copy in docs/public-profile.schema.json to remain
// byte-identical.
//
//go:embed public-profile.schema.json
var publicProfileSchema []byte

//go:embed public-profile.schema.v1.json
var publicProfileSchemaV1 []byte

var (
	compileSchemaOnce sync.Once
	compiledV2        *jsonschema.Schema
	compiledV1        *jsonschema.Schema
	compileSchemaErr  error
)

func validateSchemaJSON(raw []byte) error {
	version, err := peekSchemaVersion(raw)
	if err != nil {
		return fmt.Errorf("publicprofile: decode JSON: %w", err)
	}
	sch, err := runtimeSchema(version)
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

func peekSchemaVersion(raw []byte) (int, error) {
	var head struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return 0, err
	}
	return head.SchemaVersion, nil
}

func runtimeSchema(version int) (*jsonschema.Schema, error) {
	compileSchemaOnce.Do(func() {
		compiledV2, compileSchemaErr = compileOne(schemaResourceV2, publicProfileSchema)
		if compileSchemaErr != nil {
			return
		}
		compiledV1, compileSchemaErr = compileOne(schemaResourceV1, publicProfileSchemaV1)
	})
	if compileSchemaErr != nil {
		return nil, compileSchemaErr
	}
	switch version {
	case SchemaVersion:
		return compiledV2, nil
	case SchemaVersionV1:
		return compiledV1, nil
	default:
		return nil, fmt.Errorf("unsupported schema_version %d", version)
	}
}

func compileOne(resource string, raw []byte) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(resource, doc); err != nil {
		return nil, err
	}
	return compiler.Compile(resource)
}
