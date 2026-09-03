// Package api owns the versioned HTTP contract for Cooked.
package api

import _ "embed"

// OpenAPISpec is embedded so deployed binaries can serve the exact contract
// they were built and tested against.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
