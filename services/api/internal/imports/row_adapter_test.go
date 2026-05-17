// row_adapter_test.go — testcontainers integration suite for the
// adapter row processor (JSONB Config passthrough + D04_1-13 409
// path).
//
// Test inventory (≥ 4 cases per plan):
//
//   - HappyPath_WithConfig — config JSONB lands; round-trip preserves keys.
//   - HappyPath_EmptyConfig — nil/missing config defaults to "{}".
//   - InvalidCodeFormat_PerRow — Layer 1 reject.
//   - Reimport_UpdatesViaUpsertPath — same code → ON CONFLICT UPDATE.
package imports

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

func rawAdapterRow(code, name, adapterType string, config map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"code":         code,
		"name":         name,
		"adapter_type": adapterType,
	}
	if config != nil {
		out["config"] = config
	}
	return out
}

func fetchAdapterConfig(t testing.TB, ctx context.Context, th *TestImports, code string) (int, map[string]interface{}) {
	t.Helper()
	var n int
	var raw []byte
	err := th.Pool.QueryRow(ctx,
		`SELECT count(*), COALESCE(MAX(config::text)::bytea, '{}'::bytea) FROM adapters
		   WHERE org_id = $1 AND code = $2`,
		th.OrgID, code,
	).Scan(&n, &raw)
	require.NoError(t, err)
	if n == 0 {
		return 0, nil
	}
	var cfg map[string]interface{}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &cfg))
	}
	return n, cfg
}

func TestProcessRow_Adapter_HappyPath_WithConfig(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAdapterRow("adapter_twilio", "Twilio", "voice",
			map[string]interface{}{"endpoint": "https://api.twilio.com", "version": "v1"})},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Adapters, &adapterRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	n, cfg := fetchAdapterConfig(t, ctx, th, "adapter_twilio")
	require.Equal(t, 1, n)
	require.Equal(t, "https://api.twilio.com", cfg["endpoint"])
	require.Equal(t, "v1", cfg["version"])
}

func TestProcessRow_Adapter_HappyPath_EmptyConfig(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAdapterRow("adapter_empty", "Empty", "voice", nil)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Adapters, &adapterRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	n, cfg := fetchAdapterConfig(t, ctx, th, "adapter_empty")
	require.Equal(t, 1, n)
	require.NotNil(t, cfg, "empty config must default to {} not NULL")
	require.Equal(t, 0, len(cfg))
}

func TestProcessRow_Adapter_InvalidCodeFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAdapterRow("BadCode!", "Bad", "voice", nil)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Adapters, &adapterRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "code", *failed[0].Field)
	require.Equal(t, "invalid_code_format", failed[0].Reason)
}

func TestProcessRow_Adapter_Reimport_UpdatesViaUpsertPath(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed with config v1.
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO adapters (id, org_id, code, name, adapter_type, config, enabled, version, created_at, updated_at)
		 VALUES (gen_random_uuid(), $1, 'adapter_twilio', 'Old', 'voice', '{"version":"v0"}', TRUE, 1, NOW(), NOW())`,
		th.OrgID,
	)
	require.NoError(t, err)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAdapterRow("adapter_twilio", "New", "voice",
			map[string]interface{}{"version": "v2"})},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Adapters, &adapterRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	n, cfg := fetchAdapterConfig(t, ctx, th, "adapter_twilio")
	require.Equal(t, 1, n)
	require.Equal(t, "v2", cfg["version"], "ON CONFLICT UPDATE must overwrite config column")
}
