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
// (flow_code, version_number) race. Maps to 409 (publish) / 400 (rollback).
// errVersionNotFound: rollback target version_number does not exist.
var (
	errBindingConflict = errors.New("flowrt: binding conflict")
	errVersionNotFound = errors.New("flowrt: flow version not found")
)

type activation struct {
	flowID    pgtype.UUID
	flowCode  string
	channel   string
	entryCode string
	expected  *api.UUIDv7
	graph     []byte
	plan      []byte
}

// activate writes the immutable flow_version and flips the binding in one
// transaction, so route requests never observe two active versions for one
// binding and the partial unique index is never transiently violated.
func (e *Endpoints) activate(ctx context.Context, orgID uuid.UUID, a activation) (generated.FlowVersion, generated.FlowEntryBinding, error) {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	if err := checkExpectedBinding(ctx, qtx, orgID, a.channel, a.entryCode, a.expected); err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
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

	binding, err := bindActive(ctx, qtx, orgID, a.channel, a.entryCode, version.ID, a.flowCode)
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

// rollback re-activates an existing published version for the binding. It
// writes no new version; only the binding flips.
func (e *Endpoints) rollback(ctx context.Context, orgID uuid.UUID, t rollbackTarget) (generated.FlowVersion, generated.FlowEntryBinding, error) {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	target, err := qtx.GetFlowVersionByNumber(ctx, generated.GetFlowVersionByNumberParams{
		OrgID: pgUUID(orgID), FlowCode: t.flowCode, VersionNumber: int32(t.versionNumber),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, errVersionNotFound
	}
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}

	if err := checkExpectedBinding(ctx, qtx, orgID, t.channel, t.entryCode, t.expected); err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}

	binding, err := bindActive(ctx, qtx, orgID, t.channel, t.entryCode, target.ID, t.flowCode)
	if err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return generated.FlowVersion{}, generated.FlowEntryBinding{}, err
	}
	return target, binding, nil
}

// checkExpectedBinding enforces the optional expected_current_flow_version_id
// guard: the caller asserts the binding it is replacing. A mismatch (or no
// active binding when one was expected) is a conflict, not a clobber.
func checkExpectedBinding(ctx context.Context, qtx *generated.Queries, orgID uuid.UUID, channel, entryCode string, expected *api.UUIDv7) error {
	if expected == nil {
		return nil
	}
	cur, err := qtx.GetActiveBinding(ctx, generated.GetActiveBindingParams{
		OrgID: pgUUID(orgID), Channel: channel, EntryCode: entryCode,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errBindingConflict
	}
	if err != nil {
		return err
	}
	if apiUUID(cur.FlowVersionID) != uuid.UUID(*expected) {
		return errBindingConflict
	}
	return nil
}

// bindActive deactivates the current active binding and inserts the new one in
// the same transaction, satisfying ux_flow_bindings_active throughout.
func bindActive(ctx context.Context, qtx *generated.Queries, orgID uuid.UUID, channel, entryCode string, flowVersionID pgtype.UUID, flowCode string) (generated.FlowEntryBinding, error) {
	if _, err := qtx.DeactivateActiveBinding(ctx, generated.DeactivateActiveBindingParams{
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
