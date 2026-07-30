package config_test

import (
	"net/url"
	"testing"

	"github.com/Gabrielbsb21/restock-priority-service/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests do not call t.Parallel: they use t.Setenv, which the testing package
// forbids in parallel tests because the environment is process-wide.

// clearEnv removes every variable Load reads, so a value inherited from the developer's
// shell cannot decide the outcome of a case.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"PORT", "GIN_MODE", "DATABASE_URL",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
	} {
		t.Setenv(key, "")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "debug", cfg.GinMode)
	assert.Equal(t,
		"postgres://postgres:postgres@localhost:5432/restock_priority?sslmode=disable",
		cfg.DatabaseURL)
}

func TestLoad_ExplicitDatabaseURLWins(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://someone@db.internal:6432/other?sslmode=require")
	t.Setenv("DB_HOST", "ignored")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "postgres://someone@db.internal:6432/other?sslmode=require", cfg.DatabaseURL)
}

// TestLoad_EscapesCredentials is the regression gate for the DSN being assembled by
// net/url. Assembled by hand, the @ in the password ends the userinfo section early and
// "secret" becomes the host — the service would then connect somewhere else entirely,
// with no error to show for it.
func TestLoad_EscapesCredentials(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_USER", "app/user")
	t.Setenv("DB_PASSWORD", "p@ss:w/ord?#1")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_PORT", "6432")
	t.Setenv("DB_NAME", "restock")

	cfg, err := config.Load()
	require.NoError(t, err)

	parsed, err := url.Parse(cfg.DatabaseURL)
	require.NoError(t, err, "the assembled DSN must be a parseable URL")

	assert.Equal(t, "db.internal:6432", parsed.Host)
	assert.Equal(t, "/restock", parsed.Path)
	assert.Equal(t, "app/user", parsed.User.Username())

	password, set := parsed.User.Password()
	require.True(t, set)
	assert.Equal(t, "p@ss:w/ord?#1", password, "the password must survive the round trip intact")
}

// TestLoad_IPv6Host covers the other reason the host is not concatenated: a bare IPv6
// literal without brackets is not a valid URL authority.
func TestLoad_IPv6Host(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_HOST", "::1")

	cfg, err := config.Load()
	require.NoError(t, err)

	parsed, err := url.Parse(cfg.DatabaseURL)
	require.NoError(t, err)
	assert.Equal(t, "[::1]:5432", parsed.Host)
}

func TestLoad_GinMode(t *testing.T) {
	tests := []struct {
		mode    string
		wantErr bool
	}{
		{mode: "debug"},
		{mode: "release"},
		{mode: "test"},
		// gin's release mode is spelled "release"; this is the typo that used to panic.
		{mode: "production", wantErr: true},
		{mode: "Debug", wantErr: true},
		{mode: "anything", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("GIN_MODE", tc.mode)

			cfg, err := config.Load()

			if tc.wantErr {
				require.Error(t, err, "an unknown mode must fail configuration, not gin.SetMode")
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), "GIN_MODE")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.mode, cfg.GinMode)
		})
	}
}
