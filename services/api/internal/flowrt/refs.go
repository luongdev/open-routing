package flowrt

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// dbRefs is the DB-backed runtime.CatalogRefs. The interface's Has* methods
// return only bool (no error), so an infrastructure failure during a lookup is
// captured on `err` and surfaced by the caller after validation via Err(),
// rather than being swallowed as a false "not found". Results are memoized so a
// code referenced by several nodes costs one query.
type dbRefs struct {
	ctx   context.Context
	q     *generated.Queries
	orgID pgtype.UUID
	memo  map[string]bool
	err   error
}

func newDBRefs(ctx context.Context, q *generated.Queries, orgID pgtype.UUID) *dbRefs {
	return &dbRefs{ctx: ctx, q: q, orgID: orgID, memo: map[string]bool{}}
}

func (r *dbRefs) Err() error { return r.err }

// exists memoizes per code. lookup returns (enabled, err); a soft-deleted
// (disabled) catalog row must NOT validate as a usable reference, so existence
// is "row found AND enabled" — the Get*ByCode queries don't filter enabled, so
// we check the returned flag here (cross-AI review MED-3).
func (r *dbRefs) exists(kind, code string, lookup func() (bool, error)) bool {
	if r.err != nil {
		return false
	}
	key := kind + ":" + code
	if v, ok := r.memo[key]; ok {
		return v
	}
	enabled, err := lookup()
	switch {
	case err == nil:
		r.memo[key] = enabled
		return enabled
	case errors.Is(err, pgx.ErrNoRows):
		r.memo[key] = false
		return false
	default:
		r.err = err
		return false
	}
}

func (r *dbRefs) HasQueue(code string) bool {
	return r.exists("queue", code, func() (bool, error) {
		row, e := r.q.GetQueueByCode(r.ctx, generated.GetQueueByCodeParams{OrgID: r.orgID, Code: code})
		return row.Enabled, e
	})
}

func (r *dbRefs) HasSkill(code string) bool {
	return r.exists("skill", code, func() (bool, error) {
		row, e := r.q.GetSkillByCode(r.ctx, generated.GetSkillByCodeParams{OrgID: r.orgID, Code: code})
		return row.Enabled, e
	})
}

func (r *dbRefs) HasChannel(code string) bool {
	return r.exists("channel", code, func() (bool, error) {
		row, e := r.q.GetChannelByCode(r.ctx, generated.GetChannelByCodeParams{OrgID: r.orgID, Code: code})
		return row.Enabled, e
	})
}

func (r *dbRefs) HasAdapter(code string) bool {
	return r.exists("adapter", code, func() (bool, error) {
		row, e := r.q.GetAdapterByCode(r.ctx, generated.GetAdapterByCodeParams{OrgID: r.orgID, Code: code})
		return row.Enabled, e
	})
}

func (r *dbRefs) HasBreakReason(code string) bool {
	return r.exists("break_reason", code, func() (bool, error) {
		row, e := r.q.GetBreakReasonByCode(r.ctx, generated.GetBreakReasonByCodeParams{OrgID: r.orgID, Code: code})
		return row.Enabled, e
	})
}
