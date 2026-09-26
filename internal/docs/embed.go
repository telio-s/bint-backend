package docs

import _ "embed"

var (
	//go:embed openapi.yaml
	OpenAPI []byte

	//go:embed scalar.html
	ScalarHTML []byte
)
