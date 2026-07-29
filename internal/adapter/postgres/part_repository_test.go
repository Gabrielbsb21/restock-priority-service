package postgres_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	adapterPG "github.com/Gabrielbsb21/restock-priority-service/internal/adapter/postgres"
	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// These tests assert the SQL this adapter emits, not the rows PostgreSQL stores.
//
// GORM's DryRun builds a statement without executing it, and the PostgreSQL
// dialector's Initialize only parses the DSN, so no server is contacted. That makes
// the statement shape a cheap regression gate. Behaviour that needs a real database
// is covered by the manual verification recorded in SPEC-001.

// immutableColumns must never appear in an UPDATE's SET clause. The identifier is
// fixed for the lifetime of a part, and created_at records when it first existed.
var immutableColumns = []string{"id", "created_at"}

// newDryRunDB opens a GORM handle that builds SQL without connecting.
func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		postgres.New(postgres.Config{DSN: "postgres://user:pass@127.0.0.1:1/none?sslmode=disable"}),
		&gorm.Config{
			DryRun:               true,
			DisableAutomaticPing: true,
			// Without this, the default write transaction tries to BEGIN against a
			// connection that does not exist, and the chain aborts before the
			// statement is built.
			SkipDefaultTransaction: true,
			Logger:                 gormlogger.Discard,
		},
	)
	require.NoError(t, err)

	return db
}

// captureSQL records the statement built by the next update.
func captureSQL(t *testing.T, db *gorm.DB, captured *string) {
	t.Helper()

	err := db.Callback().Update().After("gorm:update").Register("test:capture_sql", func(tx *gorm.DB) {
		*captured = tx.Statement.SQL.String()
	})
	require.NoError(t, err)
}

// mutablePartColumns derives, from PartModel itself, the columns a full replacement
// has to write. Deriving them rather than restating them is the point: a new field on
// the model shows up here automatically, so the assertions below fail if the adapter
// is not updated to write it.
func mutablePartColumns(t *testing.T) []string {
	t.Helper()

	naming := schema.NamingStrategy{}
	skip := map[string]bool{"id": true, "created_at": true, "updated_at": true}

	modelType := reflect.TypeOf(adapterPG.PartModel{})
	columns := make([]string, 0, modelType.NumField())

	for i := range modelType.NumField() {
		column := naming.ColumnName("", modelType.Field(i).Name)
		if skip[column] {
			continue
		}
		columns = append(columns, column)
	}

	require.NotEmpty(t, columns)

	return columns
}

// setClause isolates the assignment list so a column named in SET is not confused
// with the same column named in WHERE.
func setClause(t *testing.T, statement string) string {
	t.Helper()

	_, afterSet, found := strings.Cut(statement, " SET ")
	require.True(t, found, "expected an UPDATE with a SET clause, got: %s", statement)

	beforeWhere, _, found := strings.Cut(afterSet, " WHERE ")
	require.True(t, found, "expected a WHERE clause, got: %s", statement)

	return beforeWhere
}

func partWith(currentStock, minimumStock int64, leadTimeDays int32, sales, cost string) *domain.Part {
	return &domain.Part{
		ID:                uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		Name:              "Oil Filter X",
		Category:          "engine",
		CurrentStock:      currentStock,
		MinimumStock:      minimumStock,
		AverageDailySales: decimal.RequireFromString(sales),
		LeadTimeDays:      leadTimeDays,
		UnitCost:          decimal.RequireFromString(cost),
		CriticalityLevel:  3,
	}
}

// TestPartRepository_UpdateWritesEveryMutableColumn is the regression gate for the
// defect where a full replacement silently kept the previous value.
//
// Updates with a struct skips zero-valued fields unless the column is named, so
// dropping the explicit column list would remove exactly the zero-valued assignments
// from SET while every other test still passed.
func TestPartRepository_UpdateWritesEveryMutableColumn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		part *domain.Part
	}{
		{
			name: "non-zero values",
			part: partWith(15, 20, 5, "4", "18.50"),
		},
		{
			// The case that used to be lost. Every one of these is a zero value.
			name: "zero stock, zero minimum, zero lead time, zero sales, zero cost",
			part: partWith(0, 0, 0, "0", "0"),
		},
		{
			name: "negative current stock",
			part: partWith(-25, 0, 0, "0", "0"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := newDryRunDB(t)

			var statement string
			captureSQL(t, db, &statement)

			// DryRun executes nothing, so no row is reported as affected and Update
			// reports the part as missing. The statement it built is what matters.
			_ = adapterPG.NewPartRepository(db).Update(context.Background(), tc.part)

			assignments := setClause(t, statement)

			for _, column := range mutablePartColumns(t) {
				assert.Contains(t, assignments, `"`+column+`"=`,
					"a full replacement must write %s even when its value is zero", column)
			}
		})
	}
}

func TestPartRepository_UpdateNeverWritesImmutableColumns(t *testing.T) {
	t.Parallel()

	db := newDryRunDB(t)

	var statement string
	captureSQL(t, db, &statement)

	_ = adapterPG.NewPartRepository(db).Update(context.Background(), partWith(15, 20, 5, "4", "18.50"))

	assignments := setClause(t, statement)

	for _, column := range immutableColumns {
		assert.NotContains(t, assignments, `"`+column+`"=`, "%s must not be reassigned by an update", column)
	}

	// The audit column is the one exception: it is expected to advance on every write.
	assert.Contains(t, assignments, `"updated_at"=`)
}

// TestPartModel_MapsEveryDomainField guards the translation in both directions, so a
// new field on domain.Part cannot be persisted as a zero value by omission.
func TestPartModel_MapsEveryDomainField(t *testing.T) {
	t.Parallel()

	original := partWith(-7, 20, 5, "2.5", "18.50")

	roundTripped := adapterPG.FromDomain(original).ToDomain()

	assert.Equal(t, original.ID, roundTripped.ID)
	assert.Equal(t, original.Name, roundTripped.Name)
	assert.Equal(t, original.Category, roundTripped.Category)
	assert.Equal(t, original.CurrentStock, roundTripped.CurrentStock)
	assert.Equal(t, original.MinimumStock, roundTripped.MinimumStock)
	assert.Equal(t, original.LeadTimeDays, roundTripped.LeadTimeDays)
	assert.Equal(t, original.CriticalityLevel, roundTripped.CriticalityLevel)
	assert.True(t, original.AverageDailySales.Equal(roundTripped.AverageDailySales))
	assert.True(t, original.UnitCost.Equal(roundTripped.UnitCost))

	// reflect.DeepEqual would compare decimal internals, so the count is checked
	// separately: if domain.Part grows a field, this fails and the mappers get read.
	assert.Equal(t, 9, reflect.TypeOf(domain.Part{}).NumField(),
		"domain.Part changed shape; update FromDomain, ToDomain and mutableColumns")
}
