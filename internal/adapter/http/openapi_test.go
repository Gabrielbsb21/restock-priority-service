package http_test

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/api"
	"github.com/Gabrielbsb21/restock-priority-service/internal/adapter/memory"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The OpenAPI document is hand-written, so these tests are what keep it honest. They
// are the reason annotations on the handlers are not needed: a stale annotation fails
// nothing, whereas a route missing from the spec fails here.

// undocumentedRoutes are served but deliberately absent from the contract: they deliver
// the documentation itself rather than being part of the API surface it describes.
var undocumentedRoutes = map[string]bool{
	"GET /openapi.yaml":    true,
	"GET /docs":            true,
	"GET /docs/{filepath}": true,
}

// httpMethods are the keys under a path item that denote an operation. Anything else,
// such as a path-level `parameters` list, is not one.
var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

func loadSpec(t *testing.T) map[string]any {
	t.Helper()

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(api.OpenAPISpec, &doc), "the embedded OpenAPI document must be valid YAML")

	return doc
}

// specOperations returns every documented operation as "METHOD /path".
func specOperations(t *testing.T, doc map[string]any) []string {
	t.Helper()

	paths, ok := doc["paths"].(map[string]any)
	require.True(t, ok, "the document must declare paths")

	operations := make([]string, 0, len(paths))
	for path, item := range paths {
		pathItem, ok := item.(map[string]any)
		require.Truef(t, ok, "path %s must be a mapping", path)

		for key := range pathItem {
			if httpMethods[strings.ToLower(key)] {
				operations = append(operations, strings.ToUpper(key)+" "+path)
			}
		}
	}

	sort.Strings(operations)

	return operations
}

// routerOperations returns every registered route in the same notation, translating
// Gin's ":id" and "*any" placeholders into the OpenAPI "{id}" form.
func routerOperations(t *testing.T) []string {
	t.Helper()

	repo := memory.NewPartRepository()
	engine, ok := newRouter(repo, repo).(*gin.Engine)
	require.True(t, ok, "expected the router to be a *gin.Engine")

	operations := make([]string, 0, len(engine.Routes()))
	for _, route := range engine.Routes() {
		segments := strings.Split(route.Path, "/")
		for i, segment := range segments {
			if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
				segments[i] = "{" + segment[1:] + "}"
			}
		}

		operation := route.Method + " " + strings.Join(segments, "/")
		if undocumentedRoutes[operation] {
			continue
		}
		operations = append(operations, operation)
	}

	sort.Strings(operations)

	return operations
}

// TestOpenAPI_MatchesRegisteredRoutes fails in both directions: a route added without
// documenting it, and a documented operation that is not served.
func TestOpenAPI_MatchesRegisteredRoutes(t *testing.T) {
	documented := specOperations(t, loadSpec(t))
	served := routerOperations(t)

	assert.Equal(t, served, documented,
		"the OpenAPI document and the router disagree; every served route must be documented and vice versa")
}

// TestOpenAPI_UndocumentedRoutesAreAllServed keeps the exclusion list above from
// silently outliving the routes it excuses.
func TestOpenAPI_UndocumentedRoutesAreAllServed(t *testing.T) {
	repo := memory.NewPartRepository()
	engine, ok := newRouter(repo, repo).(*gin.Engine)
	require.True(t, ok)

	served := make(map[string]bool, len(engine.Routes()))
	for _, route := range engine.Routes() {
		segments := strings.Split(route.Path, "/")
		for i, segment := range segments {
			if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
				segments[i] = "{" + segment[1:] + "}"
			}
		}
		served[route.Method+" "+strings.Join(segments, "/")] = true
	}

	for excluded := range undocumentedRoutes {
		assert.Truef(t, served[excluded], "%q is excluded from the contract but is no longer served", excluded)
	}
}

// TestOpenAPI_EveryReferenceResolves catches a typo in a $ref, which would otherwise
// only show up as a broken Swagger UI in front of whoever opened it.
func TestOpenAPI_EveryReferenceResolves(t *testing.T) {
	doc := loadSpec(t)

	refs := map[string][]string{}
	collectRefs(doc, "$", refs)
	require.NotEmpty(t, refs, "expected the document to use $ref")

	for ref, sites := range refs {
		target, err := resolveRef(doc, ref)
		if !assert.NoErrorf(t, err, "%s (referenced from %s)", ref, strings.Join(sites, ", ")) {
			continue
		}
		assert.NotNilf(t, target, "%s resolves to an empty node", ref)
	}
}

// collectRefs walks the document and records every $ref with where it was found.
func collectRefs(node any, path string, into map[string][]string) {
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			if key == "$ref" {
				if ref, ok := value.(string); ok {
					into[ref] = append(into[ref], path)
				}
				continue
			}
			collectRefs(value, path+"."+key, into)
		}
	case []any:
		for i, value := range typed {
			collectRefs(value, fmt.Sprintf("%s[%d]", path, i), into)
		}
	}
}

// resolveRef follows a local JSON pointer such as "#/components/schemas/Part".
func resolveRef(doc map[string]any, ref string) (any, error) {
	pointer, found := strings.CutPrefix(ref, "#/")
	if !found {
		return nil, fmt.Errorf("only local references are supported, got %q", ref)
	}

	var current any = doc
	for _, segment := range strings.Split(pointer, "/") {
		container, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q: %q is not a mapping", ref, segment)
		}

		current, ok = container[segment]
		if !ok {
			return nil, fmt.Errorf("%q: %q does not exist", ref, segment)
		}
	}

	return current, nil
}

// TestOpenAPI_DeclaresEveryErrorCode ties the documented enum to the codes the service
// actually emits, so a new code cannot be introduced without describing it.
func TestOpenAPI_DeclaresEveryErrorCode(t *testing.T) {
	doc := loadSpec(t)

	target, err := resolveRef(doc, "#/components/schemas/Error")
	require.NoError(t, err)

	schema, ok := target.(map[string]any)
	require.True(t, ok)

	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)

	code, ok := properties["code"].(map[string]any)
	require.True(t, ok)

	rawEnum, ok := code["enum"].([]any)
	require.True(t, ok, "the error code must be documented as an enumeration")

	documented := make(map[string]bool, len(rawEnum))
	for _, value := range rawEnum {
		documented[fmt.Sprint(value)] = true
	}

	// Every code the handlers, middleware and router can return.
	for _, emitted := range []string{
		"invalid_request",
		"validation_error",
		"part_not_found",
		"not_found",
		"method_not_allowed",
		"request_too_large",
		"internal_error",
		"service_unavailable",
	} {
		assert.Truef(t, documented[emitted], "error code %q is emitted but not documented", emitted)
	}

	assert.Len(t, documented, 8, "the documented enum lists a code the service never emits")
}

func TestHTTP_ServesTheOpenAPIDocument(t *testing.T) {
	router := setupRouter()

	recorder := do(t, router, http.MethodGet, "/openapi.yaml", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "application/yaml")

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(recorder.Body.Bytes(), &doc), "the served document must be valid YAML")
	assert.Equal(t, "3.0.3", doc["openapi"])
}

func TestHTTP_ServesSwaggerUI(t *testing.T) {
	router := setupRouter()

	t.Run("the bare path redirects to the directory", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/docs", nil)

		assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
		assert.Equal(t, "/docs/", recorder.Header().Get("Location"))
	})

	t.Run("the directory serves the UI page", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/docs/", nil)

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "swagger-ui")
	})

	t.Run("the UI is served from embedded assets", func(t *testing.T) {
		for _, asset := range []string{
			"/docs/swagger-ui.css",
			"/docs/swagger-ui-bundle.js",
			"/docs/swagger-ui-standalone-preset.js",
			"/docs/favicon-32x32.png",
		} {
			recorder := do(t, router, http.MethodGet, asset, nil)
			assert.Equalf(t, http.StatusOK, recorder.Code, "%s must be served from the binary", asset)
			assert.NotZerof(t, recorder.Body.Len(), "%s must not be empty", asset)
		}
	})

	// http.FileServer canonicalizes an explicit index.html to its directory, so /docs/
	// is the single canonical entry point.
	t.Run("index.html canonicalizes to the directory", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/docs/index.html", nil)

		assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
		assert.Equal(t, "./", recorder.Header().Get("Location"))
	})

	t.Run("an unknown asset is not found", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/docs/nope.js", nil)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	// Swagger UI ships an initializer pointing at the public petstore demo, so this is
	// what makes the page document *this* service.
	t.Run("the UI is pointed at our own document and not the petstore", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/docs/swagger-initializer.js", nil)

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `url: "/openapi.yaml"`,
			"the UI must load the embedded spec")
		assert.NotContains(t, recorder.Body.String(), "petstore",
			"the shipped initializer must be overridden")
	})
}
