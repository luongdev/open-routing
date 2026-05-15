// schema_test.go proves the schema-layer constraints for FOUND-02 and
// FOUND-06 directly against the testcontainer Postgres. These tests use
// the raw shared pool (NOT orgDB) on purpose — we are testing what
// Postgres itself enforces, not what the application layer enforces:
//
//   - FOUND-02: org_id UUID NOT NULL on _scaffold (and every org-scoped
//     table in later phases). Verified via information_schema.columns.
//   - FOUND-06: UNIQUE(org_id, external_id) on _scaffold. Verified by
//     programmatically inserting a duplicate (org_id, external_id) and
//     catching SQLSTATE 23505 (unique_violation).
//
// The orgDB validator tests live in internal/db/sqlcheck_test.go and
// orgkey_test.go — those exercise the application enforcement layer.
// Together with this file the proof covers both layers of the defense in
// depth model (D-01 + the schema constraint).
package isolation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// TestSchema_HasOrgIdNotNull verifies _scaffold.org_id is UUID NOT NULL
// (FOUND-02). Queries information_schema.columns directly so the assertion
// is independent of the migration file's text content — if a future
// migration accidentally drops NOT NULL, this test fails before the API
// has a chance to silently accept NULL org_ids.
func TestSchema_HasOrgIdNotNull(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	ctx := context.Background()
	var isNullable, dataType string
	err := sharedPool.QueryRow(ctx, `
		SELECT is_nullable, data_type
		FROM information_schema.columns
		WHERE table_name = '_scaffold' AND column_name = 'org_id'
	`).Scan(&isNullable, &dataType)
	require.NoError(t, err)
	require.Equal(t, "NO", isNullable, "FOUND-02: org_id must be NOT NULL")
	require.Equal(t, "uuid", dataType, "FOUND-02: org_id must be UUID type")
}

// TestScaffold_UniqueOrgExternalId verifies FOUND-06's
// UNIQUE(org_id, external_id) constraint. Strategy: insert one row via the
// raw pool (BYPASSING orgDB so the validator does not reject the raw SQL
// for missing the org_id filter — we are testing the schema, not the
// application). Then insert a second row with the same (org_id,
// external_id) tuple and assert SQLSTATE 23505 surfaces.
//
// 23505 is Postgres's unique_violation code (per the official errcode
// table). pgx v5 exposes it via pgconn.PgError.SQLState().
func TestScaffold_UniqueOrgExternalId(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	ctx := context.Background()
	org := uuid.Must(uuid.NewV7())

	// First insert — should succeed.
	_, err := sharedPool.Exec(ctx,
		`INSERT INTO _scaffold (id, org_id, external_id, name) VALUES ($1, $2, $3, $4)`,
		uuid.Must(uuid.NewV7()), org, "ext-unique-test", "first",
	)
	require.NoError(t, err)

	// Second insert with same (org_id, external_id) — must fail with 23505.
	_, err = sharedPool.Exec(ctx,
		`INSERT INTO _scaffold (id, org_id, external_id, name) VALUES ($1, $2, $3, $4)`,
		uuid.Must(uuid.NewV7()), org, "ext-unique-test", "second",
	)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected *pgconn.PgError; got %T: %v", err, err)
	require.Equal(t, "23505", pgErr.SQLState(),
		"FOUND-06: duplicate (org_id, external_id) must trigger Postgres unique_violation")
}

// TestScaffold_UniqueOrgExternalId_DifferentOrgsCanShareExternalId verifies
// FOUND-06's nuance: the UNIQUE constraint is SCOPED to (org_id, external_id),
// NOT a global UNIQUE(external_id). Two different orgs MUST be able to use
// the same external_id value without colliding. This is the data property
// the isolation suite's overlapping-external_id seeds rely on.
func TestScaffold_UniqueOrgExternalId_DifferentOrgsCanShareExternalId(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	ctx := context.Background()
	orgA := uuid.Must(uuid.NewV7())
	orgB := uuid.Must(uuid.NewV7())

	_, err := sharedPool.Exec(ctx,
		`INSERT INTO _scaffold (id, org_id, external_id, name) VALUES ($1, $2, $3, $4)`,
		uuid.Must(uuid.NewV7()), orgA, "ext-shared-cross", "A",
	)
	require.NoError(t, err)
	_, err = sharedPool.Exec(ctx,
		`INSERT INTO _scaffold (id, org_id, external_id, name) VALUES ($1, $2, $3, $4)`,
		uuid.Must(uuid.NewV7()), orgB, "ext-shared-cross", "B",
	)
	require.NoError(t, err,
		"FOUND-06: different orgs must be able to reuse the same external_id")
}
