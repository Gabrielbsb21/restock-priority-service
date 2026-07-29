// Package api embeds the OpenAPI description of the HTTP contract so the service can
// serve it without depending on files being present at runtime.
//
// The document is hand-written rather than generated from annotations, and
// TestOpenAPI_MatchesRegisteredRoutes fails if it and the router ever disagree.
package api

import _ "embed"

// OpenAPISpec is the OpenAPI 3.0 description of the API, served at /openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
