package runtime

import (
	"context"
	"fmt"
	"regexp"

	"github.com/luongdev/open-routing/services/api/internal/runtime/expr"
)

// varNameRe bounds a settable variable name: an identifier, optionally dotted.
// Mirrors the expr engine's variable-path grammar so a set_var name is a valid
// lookup path downstream.
var varNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// node_kinds.go holds the v0.2 executable subset. Each kind owns its config
// shape, Validate (field + catalog-reference checks), and Compile (normalize to
// a PlanStep). Execute is inherited from baseNode (Wave 3). A nil CatalogRefs
// means structure-only validation (unit tests / no DB) — reference checks are
// skipped, never panicked on.
//
// Validation issue codes are stable strings the UI surfaces verbatim:
const (
	IssueMissingField    = "missing_required_field"
	IssueMissingCatalog  = "missing_catalog_reference"
	IssueInvalidConfig   = "invalid_config"
	IssueMalformedConfig = "malformed_config"
	IssueInvalidExpr     = "invalid_expression"
)

func malformed(nodeID string, err error) []ValidationIssue {
	return []ValidationIssue{fieldIssue(nodeID, "config", IssueMalformedConfig, fmt.Sprintf("config is not valid JSON: %v", err))}
}

// checkExpr parses a non-empty expression and returns an invalid_expression
// issue on a syntax error or a semantic error (unknown function / wrong arity) —
// the publish gate rejects what would otherwise fail at runtime mid-flow.
func checkExpr(nodeID, field, src string) []ValidationIssue {
	if src == "" {
		return nil
	}
	node, perr := expr.Parse(src)
	if perr != nil {
		return []ValidationIssue{fieldIssue(nodeID, field, IssueInvalidExpr, perr.Msg)}
	}
	if err := expr.Check(node); err != nil {
		return []ValidationIssue{fieldIssue(nodeID, field, IssueInvalidExpr, err.Error())}
	}
	return nil
}

// ---- trigger ----

type triggerConfig struct {
	Channel   string `json:"channel,omitempty"`
	EntryCode string `json:"entry_code,omitempty"`
}

type triggerNode struct{ baseNode }

func (triggerNode) Validate(_ context.Context, n GraphNode, _ *Graph, refs CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[triggerConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	// The active binding (channel, entry_code) is set at publish time, so the
	// trigger's channel is an optional hint; validate it only when present.
	if cfg.Channel != "" && refs != nil && !refs.HasChannel(cfg.Channel) {
		return []ValidationIssue{fieldIssue(n.ID, "channel", IssueMissingCatalog, fmt.Sprintf("channel %q not found", cfg.Channel))}, nil
	}
	return nil, nil
}

func (triggerNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[triggerConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- if_else ----

type ifElseConfig struct {
	Expr string `json:"expr"`
}

type ifElseNode struct{ baseNode }

func (ifElseNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[ifElseConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	if cfg.Expr == "" {
		return []ValidationIssue{fieldIssue(n.ID, "expr", IssueMissingField, "if_else requires a condition expression")}, nil
	}
	return checkExpr(n.ID, "expr", cfg.Expr), nil
}

func (ifElseNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[ifElseConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- switch_case ----

type switchCaseConfig struct {
	Expr  string   `json:"expr"`
	Cases []string `json:"cases"`
}

type switchCaseNode struct{ baseNode }

func (switchCaseNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[switchCaseConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.Expr == "" {
		issues = append(issues, fieldIssue(n.ID, "expr", IssueMissingField, "switch_case requires a value expression"))
	}
	if len(cfg.Cases) == 0 {
		issues = append(issues, fieldIssue(n.ID, "cases", IssueMissingField, "switch_case requires at least one case"))
	}
	return append(issues, checkExpr(n.ID, "expr", cfg.Expr)...), nil
}

func (switchCaseNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[switchCaseConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- wait ----

type waitConfig struct {
	DurationMs int64 `json:"duration_ms"`
}

type waitNode struct{ baseNode }

func (waitNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[waitConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	if cfg.DurationMs <= 0 {
		return []ValidationIssue{fieldIssue(n.ID, "duration_ms", IssueInvalidConfig, "wait requires a positive duration_ms")}, nil
	}
	return nil, nil
}

func (waitNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[waitConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- match_skill ----

type matchSkillConfig struct {
	Skill          string `json:"skill"`
	MinProficiency int    `json:"min_proficiency,omitempty"`
}

type matchSkillNode struct{ baseNode }

func (matchSkillNode) Validate(_ context.Context, n GraphNode, _ *Graph, refs CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[matchSkillConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.Skill == "" {
		issues = append(issues, fieldIssue(n.ID, "skill", IssueMissingField, "match_skill requires a skill code"))
	} else if refs != nil && !refs.HasSkill(cfg.Skill) {
		issues = append(issues, fieldIssue(n.ID, "skill", IssueMissingCatalog, fmt.Sprintf("skill %q not found", cfg.Skill)))
	}
	if cfg.MinProficiency < 0 {
		issues = append(issues, fieldIssue(n.ID, "min_proficiency", IssueInvalidConfig, "min_proficiency cannot be negative"))
	}
	return issues, nil
}

func (matchSkillNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[matchSkillConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- filter ----

type filterConfig struct {
	Expr string `json:"expr"`
}

type filterNode struct{ baseNode }

func (filterNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[filterConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	if cfg.Expr == "" {
		return []ValidationIssue{fieldIssue(n.ID, "expr", IssueMissingField, "filter requires a predicate expression")}, nil
	}
	return checkExpr(n.ID, "expr", cfg.Expr), nil
}

func (filterNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[filterConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- route_queue ----

type routeQueueConfig struct {
	Queue string `json:"queue"`
}

type routeQueueNode struct{ baseNode }

func (routeQueueNode) Validate(_ context.Context, n GraphNode, _ *Graph, refs CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[routeQueueConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	if cfg.Queue == "" {
		return []ValidationIssue{fieldIssue(n.ID, "queue", IssueMissingField, "route_queue requires a queue code")}, nil
	}
	if refs != nil && !refs.HasQueue(cfg.Queue) {
		return []ValidationIssue{fieldIssue(n.ID, "queue", IssueMissingCatalog, fmt.Sprintf("queue %q not found", cfg.Queue))}, nil
	}
	return nil, nil
}

func (routeQueueNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[routeQueueConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- reservation ----

type reservationConfig struct {
	TimeoutSec  int `json:"timeout_sec"`
	MaxAttempts int `json:"max_attempts,omitempty"`
}

type reservationNode struct{ baseNode }

func (reservationNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[reservationConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.TimeoutSec <= 0 {
		issues = append(issues, fieldIssue(n.ID, "timeout_sec", IssueInvalidConfig, "reservation requires a positive timeout_sec"))
	}
	if cfg.MaxAttempts < 0 {
		issues = append(issues, fieldIssue(n.ID, "max_attempts", IssueInvalidConfig, "max_attempts cannot be negative"))
	}
	return issues, nil
}

func (reservationNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[reservationConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- fallback ----

type fallbackConfig struct {
	Reason string `json:"reason,omitempty"`
}

type fallbackNode struct{ baseNode }

func (fallbackNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	if _, err := decodeConfig[fallbackConfig](n.Config); err != nil {
		return malformed(n.ID, err), nil
	}
	return nil, nil
}

func (fallbackNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[fallbackConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- effect ----

type effectConfig struct {
	Adapter string `json:"adapter,omitempty"`
	Action  string `json:"action,omitempty"`
}

type effectNode struct{ baseNode }

func (effectNode) Validate(_ context.Context, n GraphNode, _ *Graph, refs CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[effectConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	if cfg.Adapter != "" && refs != nil && !refs.HasAdapter(cfg.Adapter) {
		return []ValidationIssue{fieldIssue(n.ID, "adapter", IssueMissingCatalog, fmt.Sprintf("adapter %q not found", cfg.Adapter))}, nil
	}
	return nil, nil
}

func (effectNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[effectConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- log ----

type logConfig struct {
	Message string `json:"message,omitempty"`
	Level   string `json:"level,omitempty"`
}

type logNode struct{ baseNode }

var logLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

func (logNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[logConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	if cfg.Level != "" && !logLevels[cfg.Level] {
		return []ValidationIssue{fieldIssue(n.ID, "level", IssueInvalidConfig, fmt.Sprintf("log level %q is not one of debug/info/warn/error", cfg.Level))}, nil
	}
	return nil, nil
}

func (logNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[logConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- end ----

type endConfig struct {
	Outcome string `json:"outcome,omitempty"`
}

type endNode struct{ baseNode }

func (endNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	if _, err := decodeConfig[endConfig](n.Config); err != nil {
		return malformed(n.ID, err), nil
	}
	return nil, nil
}

func (endNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[endConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

// ---- set_var / compute (3D-1: deterministic data nodes) ----

type setVarConfig struct {
	Name      string `json:"name"`
	ValueExpr string `json:"value_expr"`
}
type setVarNode struct{ baseNode }

func (setVarNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[setVarConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	switch {
	case cfg.Name == "":
		issues = append(issues, fieldIssue(n.ID, "name", IssueInvalidConfig, "set_var requires a variable name"))
	case !varNameRe.MatchString(cfg.Name):
		issues = append(issues, fieldIssue(n.ID, "name", IssueInvalidConfig, "variable name must be an identifier (optionally dotted)"))
	}
	issues = append(issues, checkExpr(n.ID, "value_expr", cfg.ValueExpr)...)
	return issues, nil
}

func (setVarNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[setVarConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

type computeConfig struct {
	Expr string `json:"expr"`
	Var  string `json:"var,omitempty"`
}
type computeNode struct{ baseNode }

func (computeNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[computeConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.Expr == "" {
		issues = append(issues, fieldIssue(n.ID, "expr", IssueInvalidConfig, "compute requires an expression"))
	} else {
		issues = append(issues, checkExpr(n.ID, "expr", cfg.Expr)...)
	}
	if cfg.Var != "" && !varNameRe.MatchString(cfg.Var) {
		issues = append(issues, fieldIssue(n.ID, "var", IssueInvalidConfig, "variable name must be an identifier (optionally dotted)"))
	}
	return issues, nil
}

func (computeNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[computeConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}
