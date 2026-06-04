package runtime

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/luongdev/open-routing/services/api/internal/runtime/expr"
)

// varNameRe bounds a settable variable name: an identifier, optionally dotted.
// Mirrors the expr engine's variable-path grammar so a set_var name is a valid
// lookup path downstream.
var varNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// reservedVarNames are expr keywords the lexer parses as bool/operator tokens,
// so a variable so named could be stored but never referenced. Single-segment
// names matching these (case-insensitive) are rejected.
var reservedVarNames = map[string]struct{}{"true": {}, "false": {}, "and": {}, "or": {}, "not": {}}

// validVarName reports whether name is a usable variable identifier.
func validVarName(name string) bool {
	if !varNameRe.MatchString(name) {
		return false
	}
	if !strings.Contains(name, ".") {
		if _, reserved := reservedVarNames[strings.ToLower(name)]; reserved {
			return false
		}
	}
	return true
}

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
		issues = append(issues, fieldIssue(n.ID, "name", IssueMissingField, "set_var requires a variable name"))
	case !validVarName(cfg.Name):
		issues = append(issues, fieldIssue(n.ID, "name", IssueInvalidConfig, "variable name must be an identifier (optionally dotted) and not a reserved word"))
	}
	if cfg.ValueExpr == "" {
		issues = append(issues, fieldIssue(n.ID, "value_expr", IssueMissingField, "set_var requires a value expression (use \"\" for an empty string)"))
	} else {
		issues = append(issues, checkExpr(n.ID, "value_expr", cfg.ValueExpr)...)
	}
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
	if cfg.Var != "" && !validVarName(cfg.Var) {
		issues = append(issues, fieldIssue(n.ID, "var", IssueInvalidConfig, "variable name must be an identifier (optionally dotted) and not a reserved word"))
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

// ---- 3D-2 control-flow region owners (Execute handled by the executor's
// region runner, not Node.Execute; here: config + Validate + Compile only) ----

type loopForConfig struct {
	ArrayExpr string `json:"array_expr"`
	ItemVar   string `json:"item_var,omitempty"`
	IndexVar  string `json:"index_var,omitempty"`
	MaxIter   int    `json:"max_iter,omitempty"`
}
type loopForNode struct{ baseNode }

func (loopForNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[loopForConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.ArrayExpr == "" {
		issues = append(issues, fieldIssue(n.ID, "array_expr", IssueMissingField, "loop_for requires an array expression"))
	} else {
		issues = append(issues, checkExpr(n.ID, "array_expr", cfg.ArrayExpr)...)
	}
	for _, fv := range []struct{ field, v string }{{"item_var", cfg.ItemVar}, {"index_var", cfg.IndexVar}} {
		if fv.v != "" && !validVarName(fv.v) {
			issues = append(issues, fieldIssue(n.ID, fv.field, IssueInvalidConfig, "variable name must be an identifier and not a reserved word"))
		}
	}
	if cfg.MaxIter < 0 || cfg.MaxIter > 10000 {
		issues = append(issues, fieldIssue(n.ID, "max_iter", IssueInvalidConfig, "max_iter must be between 0 and 10000"))
	}
	return issues, nil
}

func (loopForNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[loopForConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

type loopWhileConfig struct {
	CondExpr string `json:"cond_expr"`
	MaxIter  int    `json:"max_iter"`
}
type loopWhileNode struct{ baseNode }

func (loopWhileNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[loopWhileConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	var issues []ValidationIssue
	if cfg.CondExpr == "" {
		issues = append(issues, fieldIssue(n.ID, "cond_expr", IssueMissingField, "loop_while requires a condition expression"))
	} else {
		issues = append(issues, checkExpr(n.ID, "cond_expr", cfg.CondExpr)...)
	}
	if cfg.MaxIter < 1 || cfg.MaxIter > 10000 {
		issues = append(issues, fieldIssue(n.ID, "max_iter", IssueInvalidConfig, "loop_while requires max_iter between 1 and 10000"))
	}
	return issues, nil
}

func (loopWhileNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[loopWhileConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

type parallelConfig struct{}
type parallelNode struct{ baseNode }

func (parallelNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	// Branch structure (contiguous body:i, etc.) is validated at the region level
	// (validateRegions); the config itself is empty.
	if _, err := decodeConfig[parallelConfig](n.Config); err != nil {
		return malformed(n.ID, err), nil
	}
	return nil, nil
}

func (parallelNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[parallelConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}

type tryCatchConfig struct {
	ErrorVar string `json:"error_var,omitempty"`
}
type tryCatchNode struct{ baseNode }

func (tryCatchNode) Validate(_ context.Context, n GraphNode, _ *Graph, _ CatalogRefs) ([]ValidationIssue, error) {
	cfg, err := decodeConfig[tryCatchConfig](n.Config)
	if err != nil {
		return malformed(n.ID, err), nil
	}
	if cfg.ErrorVar != "" && !validVarName(cfg.ErrorVar) {
		return []ValidationIssue{fieldIssue(n.ID, "error_var", IssueInvalidConfig, "variable name must be an identifier and not a reserved word")}, nil
	}
	return nil, nil
}

func (tryCatchNode) Compile(n GraphNode, _ *Graph) (PlanStep, error) {
	cfg, err := decodeConfig[tryCatchConfig](n.Config)
	if err != nil {
		return PlanStep{}, err
	}
	return compileConfig(n, cfg)
}
