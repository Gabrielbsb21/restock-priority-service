package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	adapterHTTP "github.com/Gabrielbsb21/restock-priority-service/internal/adapter/http"
	"github.com/Gabrielbsb21/restock-priority-service/internal/application"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

type FakePartRepository struct {
	parts map[uuid.UUID]*domain.Part
}

func NewFakePartRepository() *FakePartRepository {
	return &FakePartRepository{
		parts: make(map[uuid.UUID]*domain.Part),
	}
}

func (r *FakePartRepository) Create(ctx context.Context, part *domain.Part) error {
	r.parts[part.ID] = part
	return nil
}

func (r *FakePartRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Part, error) {
	p, ok := r.parts[id]
	if !ok {
		return nil, application.ErrPartNotFound
	}
	return p, nil
}

func (r *FakePartRepository) List(ctx context.Context, filter application.ListFilter) ([]*domain.Part, int, error) {
	var list []*domain.Part
	for _, p := range r.parts {
		if filter.Category != "" && p.Category != filter.Category {
			continue
		}
		list = append(list, p)
	}

	sort.Slice(list, func(i, j int) bool {
		aFold := strings.ToLower(list[i].Name)
		bFold := strings.ToLower(list[j].Name)
		if aFold != bFold {
			return aFold < bFold
		}
		return list[i].ID.String() < list[j].ID.String()
	})

	total := len(list)
	start := filter.Offset
	if start >= total {
		return []*domain.Part{}, total, nil
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}

	return list[start:end], total, nil
}

func (r *FakePartRepository) Update(ctx context.Context, part *domain.Part) error {
	if _, ok := r.parts[part.ID]; !ok {
		return application.ErrPartNotFound
	}
	r.parts[part.ID] = part
	return nil
}

func (r *FakePartRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := r.parts[id]; !ok {
		return application.ErrPartNotFound
	}
	delete(r.parts, id)
	return nil
}

func (r *FakePartRepository) ListAll(ctx context.Context) ([]*domain.Part, error) {
	var list []*domain.Part
	for _, p := range r.parts {
		list = append(list, p)
	}
	return list, nil
}

func setupTestRouter() (*FakePartRepository, http.Handler) {
	repo := NewFakePartRepository()
	partService := application.NewPartService(repo)
	priorityEngine := domain.NewPriorityEngine()
	priorityService := application.NewPriorityService(repo, priorityEngine)

	router := adapterHTTP.NewRouter("test", partService, priorityService, nil)
	return repo, router
}

func TestHTTP_Healthz(t *testing.T) {
	_, router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestHTTP_CreatePart_Success(t *testing.T) {
	_, router := setupTestRouter()

	body := map[string]interface{}{
		"name":              "Oil Filter X",
		"category":          "engine",
		"currentStock":      15,
		"minimumStock":      20,
		"averageDailySales": 4,
		"leadTimeDays":      5,
		"unitCost":          18.50,
		"criticalityLevel":  3,
	}
	jsonBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/parts", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NotEmpty(t, w.Header().Get("Location"))

	var res map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, "Oil Filter X", res["name"])
	assert.NotEmpty(t, res["id"])
}

func TestHTTP_CreatePart_ValidationError(t *testing.T) {
	_, router := setupTestRouter()

	body := map[string]interface{}{
		"name":              "",
		"category":          "engine",
		"currentStock":      15,
		"minimumStock":      -1,
		"averageDailySales": 4,
		"leadTimeDays":      5,
		"unitCost":          18.50,
		"criticalityLevel":  10,
	}
	jsonBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/parts", bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var res map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	errObj, ok := res["error"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "validation_error", errObj["code"])
}

func TestHTTP_RestockPriorities(t *testing.T) {
	repo, router := setupTestRouter()

	// Seed part AC-006: Stock 15, Sales 4, Lead Time 5, Min 20, Criticality 3 -> projectedStock -5, urgencyScore 75
	id := uuid.New()
	_ = repo.Create(context.Background(), &domain.Part{
		ID:                id,
		Name:              "Oil Filter X",
		Category:          "engine",
		CurrentStock:      15,
		MinimumStock:      20,
		AverageDailySales: decimal.NewFromInt(4),
		LeadTimeDays:      5,
		UnitCost:          decimal.NewFromFloat(18.50),
		CriticalityLevel:  3,
	})

	req, _ := http.NewRequest("GET", "/restock/priorities", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var res struct {
		Priorities []domain.PriorityItem `json:"priorities"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &res)
	assert.NoError(t, err)

	assert.Len(t, res.Priorities, 1)
	assert.Equal(t, id, res.Priorities[0].PartID)
	assert.True(t, decimal.NewFromInt(-5).Equal(res.Priorities[0].ProjectedStock), "projectedStock should be -5")
	assert.True(t, decimal.NewFromInt(75).Equal(res.Priorities[0].UrgencyScore), "urgencyScore should be 75")
}
