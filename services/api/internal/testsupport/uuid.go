package testsupport

import (
	"testing"

	"github.com/google/uuid"
)

// FreshOrgID returns a brand-new UUIDv7. Use at the top of every test that
// needs an org_id; never define a package-level constant.
//
// Pitfall 6 enforcement: parallel tests that share a process-wide org_id
// constant collide because their database fixtures land in the same row
// space. Each test minting its own UUIDv7 guarantees isolation in the
// shared testcontainer Postgres regardless of how many subtests run in
// parallel — the same property the production code relies on (S8: every
// org_id is unique and time-ordered).
//
// uuid.NewV7() returns (uuid.UUID, error); the error path is entropy
// exhaustion (effectively never). uuid.Must converts it to a panic in
// that case — chi.Recoverer would catch it in production; here in test
// code the panic surfaces as a test failure with a clear stack, which is
// the right operator signal.
func FreshOrgID(t testing.TB) uuid.UUID {
	t.Helper()
	return uuid.Must(uuid.NewV7())
}
