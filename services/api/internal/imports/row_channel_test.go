// row_channel_test.go — testcontainers integration suite for the
// channel row processor. Channels exercise the D-76 FK probe via
// Phase 04.1's GetQueueByCode bound to the savepoint Tx.
//
// Test inventory (≥ 5 cases per plan):
//
//   - HappyPath_WithValidQueueCode — FK probe succeeds, channel lands.
//   - HappyPath_WithoutQueueCode — nil FK → NULL column, channel lands.
//   - InvalidOwnCodeFormat_PerRow — Layer 1 reject on channel.code.
//   - InvalidQueueCodeFormat_PerRow — Layer 1 reject on default_queue_code.
//   - UnknownQueueCode_PerRow — FK probe returns ErrNoRows → invalid_reference.
package imports

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

func rawChannelRow(code, name, channelType string, defaultQueueCode *string) map[string]interface{} {
	out := map[string]interface{}{
		"code":         code,
		"name":         name,
		"channel_type": channelType,
	}
	if defaultQueueCode != nil {
		out["default_queue_code"] = *defaultQueueCode
	}
	return out
}

func fetchChannelByCode(t testing.TB, ctx context.Context, th *TestImports, code string) (int, *uuid.UUID) {
	t.Helper()
	var n int
	err := th.Pool.QueryRow(ctx,
		`SELECT count(*) FROM channels WHERE org_id = $1 AND code = $2`,
		th.OrgID, code,
	).Scan(&n)
	require.NoError(t, err)
	if n == 0 {
		return 0, nil
	}
	var qID *uuid.UUID
	err = th.Pool.QueryRow(ctx,
		`SELECT default_queue_id FROM channels WHERE org_id = $1 AND code = $2`,
		th.OrgID, code,
	).Scan(&qID)
	require.NoError(t, err)
	return n, qID
}

func seedQueueRaw(t testing.TB, ctx context.Context, th *TestImports, code, name string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO queues (id, org_id, code, name, channel_types, priority, acw_sec, enabled, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, ARRAY['voice'], 0, 0, TRUE, 1, NOW(), NOW())`,
		id, th.OrgID, code, name,
	)
	require.NoError(t, err)
	return id
}

func TestProcessRow_Channel_HappyPath_WithValidQueueCode(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	qID := seedQueueRaw(t, ctx, th, "queue_sales", "Sales")

	queueCode := "queue_sales"
	rows := []parsedRow{
		{lineNo: 1, raw: rawChannelRow("chan_voice", "Voice Channel", "voice", &queueCode)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Channels, &channelRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	n, gotQID := fetchChannelByCode(t, ctx, th, "chan_voice")
	require.Equal(t, 1, n)
	require.NotNil(t, gotQID)
	require.Equal(t, qID, *gotQID, "default_queue_id must point to the resolved queue UUID")
}

func TestProcessRow_Channel_HappyPath_WithoutQueueCode(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawChannelRow("chan_voice", "Voice Channel", "voice", nil)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Channels, &channelRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 1)
	require.Len(t, failed, 0)

	n, gotQID := fetchChannelByCode(t, ctx, th, "chan_voice")
	require.Equal(t, 1, n)
	require.Nil(t, gotQID, "default_queue_id must be NULL when default_queue_code is omitted")
}

func TestProcessRow_Channel_InvalidOwnCodeFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawChannelRow("INVALID UPPER", "Bad", "voice", nil)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Channels, &channelRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "code", *failed[0].Field)
	require.Equal(t, "invalid_code_format", failed[0].Reason)
}

func TestProcessRow_Channel_InvalidQueueCodeFormat_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	bad := "BadQueueCode!"
	rows := []parsedRow{
		{lineNo: 1, raw: rawChannelRow("chan_voice", "Voice", "voice", &bad)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Channels, &channelRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "default_queue_code", *failed[0].Field)
	require.Equal(t, "invalid_code_format", failed[0].Reason)
}

func TestProcessRow_Channel_UnknownQueueCode_PerRow(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Deliberately do NOT seed any queue — FK probe returns ErrNoRows.
	missing := "queue_nonexistent"
	rows := []parsedRow{
		{lineNo: 1, raw: rawChannelRow("chan_voice", "Voice", "voice", &missing)},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Channels, &channelRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 0)
	require.Len(t, failed, 1)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "default_queue_code", *failed[0].Field)
	require.Equal(t, "invalid_reference", failed[0].Reason)

	// Channel must NOT have landed.
	n, _ := fetchChannelByCode(t, ctx, th, "chan_voice")
	require.Equal(t, 0, n,
		"savepoint must roll back fully when FK probe fails (D-76)")
}
