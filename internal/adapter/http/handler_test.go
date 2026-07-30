package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adapterHTTP "github.com/Gabrielbsb21/restock-priority-service/internal/adapter/http"
	"github.com/Gabrielbsb21/restock-priority-service/internal/adapter/memory"
	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests do not call t.Parallel: NewRouter calls gin.SetMode, which writes
// package-level state in gin, and concurrent calls would trip the race detector.

var errRepository = errors.New("connection refused")

// failingRepository stands in for an unreachable database.
type failingRepository struct{}

func (failingRepository) Create(context.Context, *domain.Part) error { return errRepository }
func (failingRepository) GetByID(context.Context, uuid.UUID) (*domain.Part, error) {
	return nil, errRepository
}
func (failingRepository) List(context.Context, application.ListFilter) ([]*domain.Part, int, error) {
	return nil, 0, errRepository
}
func (failingRepository) Update(context.Context, *domain.Part) error { return errRepository }
func (failingRepository) Delete(context.Context, uuid.UUID) error    { return errRepository }
func (failingRepository) ListAll(context.Context) ([]*domain.Part, error) {
	return nil, errRepository
}

type failingReadiness struct{}

func (failingReadiness) CheckReadiness(context.Context) error { return errRepository }

// deadlineProbe records whether the readiness check was given a deadline.
type deadlineProbe struct{ sawDeadline bool }

func (p *deadlineProbe) CheckReadiness(ctx context.Context) error {
	_, p.sawDeadline = ctx.Deadline()
	return nil
}

func newRouter(repo application.PartRepository, readiness application.ReadinessChecker) http.Handler {
	return adapterHTTP.NewRouter(
		"test",
		application.NewPartService(repo),
		application.NewPriorityService(repo, domain.NewPriorityEngine()),
		readiness,
	)
}

// setupRouter wires the HTTP surface over the in-memory adapter.
func setupRouter() http.Handler {
	repo := memory.NewPartRepository()
	return newRouter(repo, repo)
}

// validBody is the part payload from the specification's examples.
func validBody() map[string]any {
	return map[string]any{
		"name":              "Oil Filter X",
		"category":          "engine",
		"currentStock":      15,
		"minimumStock":      20,
		"averageDailySales": 4,
		"leadTimeDays":      5,
		"unitCost":          18.50,
		"criticalityLevel":  3,
	}
}

// do issues a request. A string body is sent verbatim so malformed payloads can be
// tested; anything else is marshalled.
func do(t *testing.T, router http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	switch payload := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(payload)
	default:
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		reader = strings.NewReader(string(encoded))
	}

	req := httptest.NewRequest(method, target, reader)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	return recorder
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &decoded), "body: %s", recorder.Body.String())

	return decoded
}

// assertErrorCode checks the documented envelope shape and its stable code.
func assertErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()

	assert.Equal(t, status, recorder.Code, "body: %s", recorder.Body.String())

	body := decodeBody(t, recorder)
	envelope, ok := body["error"].(map[string]any)
	require.True(t, ok, "response must use the error envelope, got: %s", recorder.Body.String())
	assert.Equal(t, code, envelope["code"])
	assert.NotEmpty(t, envelope["message"])

	return envelope
}

// createPart posts a part and returns its identifier.
func createPart(t *testing.T, router http.Handler, body map[string]any) string {
	t.Helper()

	recorder := do(t, router, http.MethodPost, "/parts", body)
	require.Equal(t, http.StatusCreated, recorder.Code, "body: %s", recorder.Body.String())

	id, ok := decodeBody(t, recorder)["id"].(string)
	require.True(t, ok)

	return id
}

// TestHTTP_CreatePart covers AC-001.
func TestHTTP_CreatePart(t *testing.T) {
	router := setupRouter()

	recorder := do(t, router, http.MethodPost, "/parts", validBody())

	require.Equal(t, http.StatusCreated, recorder.Code, "body: %s", recorder.Body.String())

	body := decodeBody(t, recorder)
	id, ok := body["id"].(string)
	require.True(t, ok, "a generated identifier must be returned")
	_, err := uuid.Parse(id)
	assert.NoError(t, err, "the identifier must be a canonical UUID")

	assert.Equal(t, "/parts/"+id, recorder.Header().Get("Location"))
	assert.Equal(t, "Oil Filter X", body["name"])
	assert.Equal(t, "engine", body["category"])
	assert.Equal(t, float64(15), body["currentStock"])
	assert.Equal(t, 3, int(body["criticalityLevel"].(float64)))

	// Decimals are JSON numbers, not quoted strings.
	assert.Contains(t, recorder.Body.String(), `"unitCost":18.5`)
	assert.Contains(t, recorder.Body.String(), `"averageDailySales":4`)
	assert.NotContains(t, recorder.Body.String(), `"unitCost":"`)
}

func TestHTTP_CreatePart_NormalizesInput(t *testing.T) {
	router := setupRouter()

	body := validBody()
	body["name"] = "  Oil Filter X  "
	body["category"] = " engine "

	recorder := do(t, router, http.MethodPost, "/parts", body)
	require.Equal(t, http.StatusCreated, recorder.Code)

	decoded := decodeBody(t, recorder)
	assert.Equal(t, "Oil Filter X", decoded["name"])
	assert.Equal(t, "engine", decoded["category"])
}

// TestHTTP_CreatePart_ValidationErrors covers AC-004.
func TestHTTP_CreatePart_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(body map[string]any)
		wantFields []string
	}{
		{
			name:       "empty name",
			mutate:     func(b map[string]any) { b["name"] = "   " },
			wantFields: []string{"name"},
		},
		{
			name:       "negative minimum stock",
			mutate:     func(b map[string]any) { b["minimumStock"] = -1 },
			wantFields: []string{"minimumStock"},
		},
		{
			name:       "criticality above the range",
			mutate:     func(b map[string]any) { b["criticalityLevel"] = 10 },
			wantFields: []string{"criticalityLevel"},
		},
		{
			name:       "criticality below the range",
			mutate:     func(b map[string]any) { b["criticalityLevel"] = 0 },
			wantFields: []string{"criticalityLevel"},
		},
		{
			name:       "unit cost with three fractional digits",
			mutate:     func(b map[string]any) { b["unitCost"] = 10.123 },
			wantFields: []string{"unitCost"},
		},
		{
			name:       "several invalid fields at once",
			mutate:     func(b map[string]any) { b["name"] = ""; b["leadTimeDays"] = -2; b["criticalityLevel"] = 99 },
			wantFields: []string{"name", "leadTimeDays", "criticalityLevel"},
		},
		{
			name:       "missing required field",
			mutate:     func(b map[string]any) { delete(b, "currentStock") },
			wantFields: []string{"currentStock"},
		},
		{
			name: "empty body reports every field",
			mutate: func(b map[string]any) {
				for key := range b {
					delete(b, key)
				}
			},
			wantFields: []string{
				"name", "category", "currentStock", "minimumStock",
				"averageDailySales", "leadTimeDays", "unitCost", "criticalityLevel",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := setupRouter()

			body := validBody()
			tc.mutate(body)

			recorder := do(t, router, http.MethodPost, "/parts", body)
			envelope := assertErrorCode(t, recorder, http.StatusBadRequest, "validation_error")

			fields, ok := envelope["fields"].(map[string]any)
			require.True(t, ok, "field-level detail is expected here")
			for _, field := range tc.wantFields {
				assert.Contains(t, fields, field)
			}

			// Nothing may be persisted by a rejected request.
			list := do(t, router, http.MethodGet, "/parts", nil)
			require.Equal(t, http.StatusOK, list.Code)
			pagination := decodeBody(t, list)["pagination"].(map[string]any)
			assert.Equal(t, float64(0), pagination["total"])
		})
	}
}

// TestHTTP_CreatePart_RejectsHugeExponent covers AC-017, the transport-level contract
// for BR-015.
//
// The body is 143 bytes, so MaxBodyBytes never comes into play: the cost is entirely
// in the number the body describes. Before the bound existed this request answered
// 201 with a ten-megabyte body after two and a half seconds of arithmetic, and the
// part it created re-rendered on every later list and ranking call. All three
// assertions below — the status, the size, the clock — failed then and are the gate
// against reintroducing it.
func TestHTTP_CreatePart_RejectsHugeExponent(t *testing.T) {
	const budget = time.Second

	for _, field := range []string{"averageDailySales", "unitCost"} {
		t.Run(field, func(t *testing.T) {
			body := validBody()
			body[field] = json.RawMessage("1e10000000")

			start := time.Now()
			recorder := do(t, setupRouter(), http.MethodPost, "/parts", body)
			elapsed := time.Since(start)

			envelope := assertErrorCode(t, recorder, http.StatusBadRequest, "validation_error")
			fields, ok := envelope["fields"].(map[string]any)
			require.True(t, ok, "field-level detail is expected here")
			assert.Contains(t, fields, field)

			assert.Less(t, recorder.Body.Len(), 1024, "the rejection must not carry the number")
			assert.Less(t, elapsed, budget, "the request must not render the number")
		})
	}
}

// TestHTTP_MalformedRequestBodies covers AC-014.
func TestHTTP_MalformedRequestBodies(t *testing.T) {
	oversized := `{"name":"` + strings.Repeat("a", adapterHTTP.MaxBodyBytes) + `"}`

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "malformed JSON",
			body:       `{"name": "Oil Filter X",}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "not an object",
			body:       `"just a string"`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "unknown field",
			body:       `{"name":"Oil Filter X","surpriseField":1}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "trailing JSON value",
			body:       `{"name":"Oil Filter X"}{"name":"Second"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "wrong type for a field",
			body:       `{"name":"Oil Filter X","currentStock":"fifteen"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "body over the configured limit",
			body:       oversized,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantCode:   "request_too_large",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := setupRouter()

			recorder := do(t, router, http.MethodPost, "/parts", tc.body)
			envelope := assertErrorCode(t, recorder, tc.wantStatus, tc.wantCode)

			// The decoder's own message names Go types; it must not reach the client.
			assert.NotContains(t, envelope["message"], "PartWriteRequest")
			assert.NotContains(t, envelope["message"], "struct")
		})
	}
}

func TestHTTP_GetPartByID(t *testing.T) {
	router := setupRouter()
	id := createPart(t, router, validBody())

	t.Run("returns the stored part", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts/"+id, nil)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, id, decodeBody(t, recorder)["id"])
	})

	t.Run("rejects a non-UUID identifier", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts/not-a-uuid", nil)
		assertErrorCode(t, recorder, http.StatusBadRequest, "invalid_request")
	})

	// AC-005.
	t.Run("reports a missing part", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts/"+uuid.NewString(), nil)
		assertErrorCode(t, recorder, http.StatusNotFound, "part_not_found")
	})
}

// TestHTTP_UpdatePart covers FR-005 and AC-002, including the zero-value replacement
// that a struct-based SQL update silently drops.
func TestHTTP_UpdatePart(t *testing.T) {
	t.Run("replaces the part and persists zero values", func(t *testing.T) {
		router := setupRouter()
		id := createPart(t, router, validBody())

		replacement := validBody()
		replacement["name"] = "Oil Filter Y"
		replacement["currentStock"] = 0
		replacement["minimumStock"] = 0
		replacement["leadTimeDays"] = 0

		recorder := do(t, router, http.MethodPut, "/parts/"+id, replacement)
		require.Equal(t, http.StatusOK, recorder.Code, "body: %s", recorder.Body.String())

		updated := decodeBody(t, recorder)
		assert.Equal(t, id, updated["id"], "the identifier must not change")
		assert.Equal(t, "Oil Filter Y", updated["name"])

		// Re-read: the response must describe what was actually stored.
		reread := do(t, router, http.MethodGet, "/parts/"+id, nil)
		require.Equal(t, http.StatusOK, reread.Code)

		stored := decodeBody(t, reread)
		assert.Equal(t, float64(0), stored["currentStock"])
		assert.Equal(t, float64(0), stored["minimumStock"])
		assert.Equal(t, float64(0), stored["leadTimeDays"])
	})

	t.Run("accepts a negative current stock", func(t *testing.T) {
		router := setupRouter()
		id := createPart(t, router, validBody())

		replacement := validBody()
		replacement["currentStock"] = -30

		recorder := do(t, router, http.MethodPut, "/parts/"+id, replacement)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, float64(-30), decodeBody(t, recorder)["currentStock"])
	})

	t.Run("rejects a partial body", func(t *testing.T) {
		router := setupRouter()
		id := createPart(t, router, validBody())

		recorder := do(t, router, http.MethodPut, "/parts/"+id, map[string]any{"name": "Only A Name"})
		envelope := assertErrorCode(t, recorder, http.StatusBadRequest, "validation_error")

		fields := envelope["fields"].(map[string]any)
		assert.Contains(t, fields, "currentStock", "partial updates are not supported")
	})

	t.Run("rejects a non-UUID identifier", func(t *testing.T) {
		router := setupRouter()
		recorder := do(t, router, http.MethodPut, "/parts/not-a-uuid", validBody())
		assertErrorCode(t, recorder, http.StatusBadRequest, "invalid_request")
	})

	// The same strict decoding as create, on the other write endpoint.
	t.Run("rejects a malformed body", func(t *testing.T) {
		router := setupRouter()
		id := createPart(t, router, validBody())

		for _, body := range []string{
			`{"name": "Oil Filter X",}`,
			`{"name":"Oil Filter X","surpriseField":1}`,
			`{"name":"Oil Filter X"}{"name":"Second"}`,
		} {
			recorder := do(t, router, http.MethodPut, "/parts/"+id, body)
			assertErrorCode(t, recorder, http.StatusBadRequest, "invalid_request")
		}
	})

	// AC-005.
	t.Run("reports a missing part", func(t *testing.T) {
		router := setupRouter()
		recorder := do(t, router, http.MethodPut, "/parts/"+uuid.NewString(), validBody())
		assertErrorCode(t, recorder, http.StatusNotFound, "part_not_found")
	})
}

func TestHTTP_DeletePart(t *testing.T) {
	router := setupRouter()
	id := createPart(t, router, validBody())

	t.Run("deletes with an empty body", func(t *testing.T) {
		recorder := do(t, router, http.MethodDelete, "/parts/"+id, nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Zero(t, recorder.Body.Len(), "204 must carry no body")
	})

	// AC-005: repeating a successful deletion reports the part as missing.
	t.Run("a repeated delete reports the part as missing", func(t *testing.T) {
		recorder := do(t, router, http.MethodDelete, "/parts/"+id, nil)
		assertErrorCode(t, recorder, http.StatusNotFound, "part_not_found")
	})

	t.Run("rejects a non-UUID identifier", func(t *testing.T) {
		recorder := do(t, router, http.MethodDelete, "/parts/not-a-uuid", nil)
		assertErrorCode(t, recorder, http.StatusBadRequest, "invalid_request")
	})
}

func TestHTTP_ListParts_EmptyCollection(t *testing.T) {
	router := setupRouter()

	recorder := do(t, router, http.MethodGet, "/parts", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	// An empty collection is an empty array, never null.
	assert.Contains(t, recorder.Body.String(), `"items":[]`)

	pagination := decodeBody(t, recorder)["pagination"].(map[string]any)
	assert.Equal(t, float64(50), pagination["limit"], "the documented default limit")
	assert.Equal(t, float64(0), pagination["offset"])
	assert.Equal(t, float64(0), pagination["total"])
}

// TestHTTP_ListParts_CategoryFilter covers AC-003.
func TestHTTP_ListParts_CategoryFilter(t *testing.T) {
	router := setupRouter()

	for _, spec := range []struct{ name, category string }{
		{"Oil Filter", "engine"},
		{"Air Filter", "engine"},
		{"Brake Pad", "brakes"},
	} {
		body := validBody()
		body["name"], body["category"] = spec.name, spec.category
		createPart(t, router, body)
	}

	t.Run("returns only exact matches and the filtered total", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts?category=engine", nil)
		require.Equal(t, http.StatusOK, recorder.Code)

		body := decodeBody(t, recorder)
		items := body["items"].([]any)
		require.Len(t, items, 2)
		assert.Equal(t, float64(2), body["pagination"].(map[string]any)["total"])
		for _, item := range items {
			assert.Equal(t, "engine", item.(map[string]any)["category"])
		}
	})

	t.Run("trims the category before matching", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts?category=%20engine%20", nil)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Len(t, decodeBody(t, recorder)["items"].([]any), 2)
	})

	t.Run("matching is case sensitive", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts?category=ENGINE", nil)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Empty(t, decodeBody(t, recorder)["items"].([]any))
	})

	t.Run("orders by name, case-insensitively, then by identifier", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts", nil)
		require.Equal(t, http.StatusOK, recorder.Code)

		names := make([]string, 0, 3)
		for _, item := range decodeBody(t, recorder)["items"].([]any) {
			names = append(names, item.(map[string]any)["name"].(string))
		}
		assert.Equal(t, []string{"Air Filter", "Brake Pad", "Oil Filter"}, names)
	})
}

func TestHTTP_ListParts_QueryValidation(t *testing.T) {
	tests := []struct {
		query      string
		wantStatus int
	}{
		{query: "", wantStatus: http.StatusOK},
		{query: "?limit=1", wantStatus: http.StatusOK},
		{query: "?limit=100", wantStatus: http.StatusOK},
		{query: "?offset=0", wantStatus: http.StatusOK},
		{query: "?offset=9999", wantStatus: http.StatusOK},
		{query: "?limit=0", wantStatus: http.StatusBadRequest},
		{query: "?limit=-1", wantStatus: http.StatusBadRequest},
		{query: "?limit=101", wantStatus: http.StatusBadRequest},
		{query: "?limit=abc", wantStatus: http.StatusBadRequest},
		{query: "?limit=", wantStatus: http.StatusBadRequest},
		{query: "?limit=1.5", wantStatus: http.StatusBadRequest},
		{query: "?offset=-1", wantStatus: http.StatusBadRequest},
		{query: "?offset=abc", wantStatus: http.StatusBadRequest},
		// A present but empty category is invalid, not absent.
		{query: "?category=", wantStatus: http.StatusBadRequest},
		{query: "?category=%20%20", wantStatus: http.StatusBadRequest},
	}

	router := setupRouter()

	for _, tc := range tests {
		t.Run("GET /parts"+tc.query, func(t *testing.T) {
			recorder := do(t, router, http.MethodGet, "/parts"+tc.query, nil)

			if tc.wantStatus == http.StatusOK {
				assert.Equal(t, http.StatusOK, recorder.Code, "body: %s", recorder.Body.String())
				return
			}

			assertErrorCode(t, recorder, http.StatusBadRequest, "invalid_request")
		})
	}
}

func TestHTTP_ListParts_Pagination(t *testing.T) {
	router := setupRouter()

	for i := range 5 {
		body := validBody()
		body["name"] = fmt.Sprintf("Part %d", i)
		createPart(t, router, body)
	}

	recorder := do(t, router, http.MethodGet, "/parts?limit=2&offset=2", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := decodeBody(t, recorder)
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(2), pagination["limit"])
	assert.Equal(t, float64(2), pagination["offset"])
	assert.Equal(t, float64(5), pagination["total"], "the total describes the whole result, not the page")
	require.Len(t, body["items"].([]any), 2)

	t.Run("an offset past the end returns an empty page", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/parts?offset=99", nil)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"items":[]`)
		assert.Equal(t, float64(5), decodeBody(t, recorder)["pagination"].(map[string]any)["total"])
	})
}

// TestHTTP_RestockPriorities_ChallengeExample covers AC-006 end to end and pins the
// decimal encoding: the numbers must not be quoted.
func TestHTTP_RestockPriorities_ChallengeExample(t *testing.T) {
	router := setupRouter()
	id := createPart(t, router, validBody())

	recorder := do(t, router, http.MethodGet, "/restock/priorities", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	raw := recorder.Body.String()
	assert.Contains(t, raw, `"projectedStock":-5`)
	assert.Contains(t, raw, `"urgencyScore":75`)
	assert.NotContains(t, raw, `"projectedStock":"-5"`, "decimals must be JSON numbers")
	assert.NotContains(t, raw, `"urgencyScore":"75"`, "decimals must be JSON numbers")

	priorities := decodeBody(t, recorder)["priorities"].([]any)
	require.Len(t, priorities, 1)

	item := priorities[0].(map[string]any)
	assert.Equal(t, id, item["partId"])
	assert.Equal(t, "Oil Filter X", item["name"])
	assert.Equal(t, float64(15), item["currentStock"])
	assert.Equal(t, float64(-5), item["projectedStock"])
	assert.Equal(t, float64(20), item["minimumStock"])
	assert.Equal(t, float64(75), item["urgencyScore"])

	// The contract is exactly these six fields.
	assert.Len(t, item, 6)
	for _, absent := range []string{"category", "criticalityLevel", "averageDailySales", "unitCost", "expectedConsumption"} {
		assert.NotContains(t, item, absent)
	}
}

// TestHTTP_RestockPriorities_Ordering is the end-to-end ordering check: several parts
// created over HTTP, then the full ranking read back.
func TestHTTP_RestockPriorities_Ordering(t *testing.T) {
	router := setupRouter()

	seed := func(name string, currentStock, minimumStock, criticality int) {
		body := validBody()
		body["name"] = name
		body["currentStock"] = currentStock
		body["minimumStock"] = minimumStock
		body["criticalityLevel"] = criticality
		body["averageDailySales"] = 0
		body["leadTimeDays"] = 0
		createPart(t, router, body)
	}

	seed("Zulu Low", 8, 10, 1)       // shortage 2, score 2
	seed("Healthy Part", 900, 10, 5) // not eligible
	seed("Alpha High", -10, 10, 5)   // shortage 20, score 100
	seed("Mid Part", 0, 10, 3)       // shortage 10, score 30

	recorder := do(t, router, http.MethodGet, "/restock/priorities", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	names := make([]string, 0, 3)
	for _, item := range decodeBody(t, recorder)["priorities"].([]any) {
		names = append(names, item.(map[string]any)["name"].(string))
	}

	assert.Equal(t, []string{"Alpha High", "Mid Part", "Zulu Low"}, names)
	assert.NotContains(t, names, "Healthy Part", "parts that do not need restocking must be absent")
}

// TestHTTP_RestockPriorities_Empty covers AC-012.
func TestHTTP_RestockPriorities_Empty(t *testing.T) {
	t.Run("no parts at all", func(t *testing.T) {
		recorder := do(t, setupRouter(), http.MethodGet, "/restock/priorities", nil)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"priorities":[]}`, recorder.Body.String())
	})

	t.Run("no part needs restocking", func(t *testing.T) {
		router := setupRouter()

		body := validBody()
		body["currentStock"] = 9000
		createPart(t, router, body)

		recorder := do(t, router, http.MethodGet, "/restock/priorities", nil)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"priorities":[]}`, recorder.Body.String())
	})
}

// TestHTTP_RestockPriorities_HasNoQueryParameters pins that v1 ignores query strings
// rather than paginating the ranking.
func TestHTTP_RestockPriorities_HasNoQueryParameters(t *testing.T) {
	router := setupRouter()
	createPart(t, router, validBody())

	recorder := do(t, router, http.MethodGet, "/restock/priorities?limit=0&offset=999", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Len(t, decodeBody(t, recorder)["priorities"].([]any), 1)
}

// TestHTTP_RepositoryFailuresReturnTheEnvelope proves an unreachable database answers
// a documented 500 rather than panicking into an empty body.
func TestHTTP_RepositoryFailuresReturnTheEnvelope(t *testing.T) {
	router := newRouter(failingRepository{}, failingReadiness{})

	tests := []struct {
		name   string
		method string
		target string
		body   any
	}{
		{name: "create", method: http.MethodPost, target: "/parts", body: validBody()},
		{name: "get", method: http.MethodGet, target: "/parts/" + uuid.NewString()},
		{name: "list", method: http.MethodGet, target: "/parts"},
		{name: "update", method: http.MethodPut, target: "/parts/" + uuid.NewString(), body: validBody()},
		{name: "delete", method: http.MethodDelete, target: "/parts/" + uuid.NewString()},
		{name: "priorities", method: http.MethodGet, target: "/restock/priorities"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := do(t, router, tc.method, tc.target, tc.body)
			envelope := assertErrorCode(t, recorder, http.StatusInternalServerError, "internal_error")
			assert.NotContains(t, envelope["message"], errRepository.Error(), "the cause must not reach the client")
		})
	}
}

// TestHTTP_Health covers FR-008 and AC-013.
func TestHTTP_Health(t *testing.T) {
	t.Run("liveness does not depend on the database", func(t *testing.T) {
		recorder := do(t, newRouter(failingRepository{}, failingReadiness{}), http.MethodGet, "/healthz", nil)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
	})

	t.Run("readiness succeeds when the dependency answers", func(t *testing.T) {
		recorder := do(t, setupRouter(), http.MethodGet, "/readyz", nil)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `{"status":"ready"}`, recorder.Body.String())
	})

	t.Run("readiness reports an unavailable dependency", func(t *testing.T) {
		recorder := do(t, newRouter(memory.NewPartRepository(), failingReadiness{}), http.MethodGet, "/readyz", nil)
		assertErrorCode(t, recorder, http.StatusServiceUnavailable, "service_unavailable")
	})

	t.Run("readiness reports a missing dependency", func(t *testing.T) {
		recorder := do(t, newRouter(memory.NewPartRepository(), nil), http.MethodGet, "/readyz", nil)
		assertErrorCode(t, recorder, http.StatusServiceUnavailable, "service_unavailable")
	})

	// The probe must be bounded, so a hung dependency cannot hold it open.
	t.Run("readiness passes a deadline to the dependency", func(t *testing.T) {
		probe := &deadlineProbe{}

		recorder := do(t, newRouter(memory.NewPartRepository(), probe), http.MethodGet, "/readyz", nil)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.True(t, probe.sawDeadline, "the readiness check must receive a deadline")
	})
}

func TestHTTP_Routing(t *testing.T) {
	router := setupRouter()
	id := createPart(t, router, validBody())

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "unknown path",
			method:     http.MethodGet,
			target:     "/does-not-exist",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "unknown nested path",
			method:     http.MethodGet,
			target:     "/parts/" + id + "/history",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "wrong method on a collection",
			method:     http.MethodDelete,
			target:     "/parts",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "method_not_allowed",
		},
		{
			name:       "wrong method on a single part",
			method:     http.MethodPost,
			target:     "/parts/" + id,
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "method_not_allowed",
		},
		{
			name:       "partial update is not supported",
			method:     http.MethodPatch,
			target:     "/parts/" + id,
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "method_not_allowed",
		},
		{
			name:       "wrong method on the ranking",
			method:     http.MethodPost,
			target:     "/restock/priorities",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "method_not_allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := do(t, router, tc.method, tc.target, nil)
			assertErrorCode(t, recorder, tc.wantStatus, tc.wantCode)
		})
	}
}

func TestHTTP_RequestIDCorrelation(t *testing.T) {
	router := setupRouter()

	t.Run("assigns an identifier when none is supplied", func(t *testing.T) {
		recorder := do(t, router, http.MethodGet, "/healthz", nil)

		id := recorder.Header().Get("X-Request-Id")
		_, err := uuid.Parse(id)
		assert.NoError(t, err, "a generated request ID must be a UUID, got %q", id)
	})

	t.Run("reuses a valid inbound identifier", func(t *testing.T) {
		inbound := uuid.NewString()

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("X-Request-Id", inbound)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		assert.Equal(t, inbound, recorder.Header().Get("X-Request-Id"))
	})

	t.Run("replaces an inbound identifier that is not a UUID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("X-Request-Id", "not-a-uuid\nlog-forging-attempt")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		id := recorder.Header().Get("X-Request-Id")
		assert.NotContains(t, id, "log-forging-attempt")
		_, err := uuid.Parse(id)
		assert.NoError(t, err)
	})
}
