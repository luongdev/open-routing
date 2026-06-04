package flowrt

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

// errBindingConflict: the active binding changed under an expected-version
// guard, or a concurrent publish won the (channel, entry_code) /
// (flow_code, version_number) race.
// errDraftConflict: the draft version changed under the publish lock (a
// concurrent UpdateFlow landed) — the compiled graph is stale.
// errVersionNotFound: rollback target version_number does not exist.
var (
	errBindingConflict = errors.New("flowrt: binding conflict")
	errDraftConflict   = errors.New("flowrt: draft version conflict")
	errVersionNotFound = errors.New("flowrt: flow version not found")
)

type activation struct {
	flowID       pgtype.UUID
	flowCode     string
	draftVersion int
	channel      string
	entryCode    string
	expected     *api.UUIDv7
	graph        []byte
	plan         []byte
}

// activate writes the immutable flow_version and flips the binding in one
// transaction. It re-locks the draft row and re-checks the version under the
// lock (HIGH-2: a concurrent UpdateFlow must not let us publish a stale graph),
// then flips the binding via a conditional deactivate when an expected version
// is given (HIGH-1: the atomic guard against a concurrent publish/rollback
// clobber). The partial unique index keeps "at most one active" true; the
// deactivate-then-insert ordering makes "exactly one active" the postcondition.
func (e *Endpoints) activate(ctx context.Context, orgID uuid.UUID, a activation) (generated.FlowVersion, generated.FlowEntryBinding, error) {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	locked, err := qtx.LockFlowForPublish(ctx, generated.LockFlowForPublishParams{
		ID: a.flowID, OrgID: pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !locked.Enabled) {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, errDraftConflict
	}
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	if int(locked.Version) != a.draftVersion {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, errDraftConflict
	}

	next, err := qtx.NextFlowVersionNumber(ctx, generated.NextFlowVersionNumberParams{
		OrgID: pgUUID(orgID), FlowCode: a.flowCode,
	})
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}

	version, err := qtx.InsertFlowVersion(ctx, generated.InsertFlowVersionParams{
		ID:                pgUUID(uuid.Must(uuid.NewV7())),
		OrgID:             pgUUID(orgID),
		FlowID:            a.flowID,
		FlowCode:          a.flowCode,
		VersionNumber:     next,
		Graph:             a.graph,
		CompiledPlan:      a.plan,
		PlanFormatVersion: runtime.PlanFormatVersion,
	})
	if isUniqueViolation(err) {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, errBindingConflict
	}
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}

	binding, err := bindActive(ctx, qtx, orgID, a.channel, a.entryCode, version.ID, a.flowCode, a.expected)
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	return version, binding, nil
}

type rollbackTarget struct {
	flowCode      string
	channel       string
	entryCode     string
	versionNumber int
	expected      *api.UUIDv7
}

// rollback re-activates an existing published version for the binding. No new
// version is written; only the binding flips (atomically, same guard as publish).
func (e *Endpoints) rollback(ctx context.Context, orgID uuid.UUID, t rollbackTarget) (generated.FlowVersion, generated.FlowEntryBinding, error) {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	target, err := qtx.GetFlowVersionByNumber(ctx, generated.GetFlowVersionByNumberParams{
		OrgID: pgUUID(orgID), FlowCode: t.flowCode, VersionNumber: int32(t.versionNumber), //nolint:gosec // version numbers are small + bounded
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, errVersionNotFound
	}
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}

	binding, err := bindActive(ctx, qtx, orgID, t.channel, t.entryCode, target.ID, t.flowCode, t.expected)
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	return target, binding, nil
}

// bindActive deactivates the current active binding and inserts the new one in
// the same transaction. When expected is set, the deactivate is conditional on
// the binding still pointing at that version (and must affect exactly one row),
// which is the atomic optimistic guard; when nil, the publish unconditionally
// replaces whatever is active. ux_flow_bindings_active is satisfied throughout.
func bindActive(ctx context.Context, qtx *generated.Queries, orgID uuid.UUID, channel, entryCode string, flowVersionID pgtype.UUID, flowCode string, expected *api.UUIDv7) (generated.FlowEntryBinding, error) {
	if expected != nil {
		n, err := qtx.DeactivateBindingIfVersion(ctx, generated.DeactivateBindingIfVersionParams{
			OrgID: pgUUID(orgID), Channel: channel, EntryCode: entryCode,
			FlowVersionID: pgUUID(uuid.UUID(*expected)),
		})
		if err != nil {
			return generated.FlowEntryBinding{}, err
		}
		if n != 1 {
			return generated.FlowEntryBinding{}, errBindingConflict
		}
	} else if _, err := qtx.DeactivateActiveBinding(ctx, generated.DeactivateActiveBindingParams{
		OrgID: pgUUID(orgID), Channel: channel, EntryCode: entryCode,
	}); err != nil {
		return generated.FlowEntryBinding{}, err
	}

	binding, err := qtx.InsertActiveBinding(ctx, generated.InsertActiveBindingParams{
		ID:            pgUUID(uuid.Must(uuid.NewV7())),
		OrgID:         pgUUID(orgID),
		Channel:       channel,
		EntryCode:     entryCode,
		FlowVersionID: flowVersionID,
		FlowCode:      flowCode,
	})
	if isUniqueViolation(err) {
		return generated.FlowEntryBinding{}, errBindingConflict
	}
	return binding, err
}
