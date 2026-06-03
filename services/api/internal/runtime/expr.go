package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// expr.go is a deliberately tiny condition language for v0.2 if_else /
// switch_case / filter. It is NOT a general expression engine (that is an
// explicit v0.2 deferral) — the grammar is exactly:
//
//	condition := varpath                      // truthy test
//	           | varpath OP literal           // comparison
//	OP        := == | != | <= | >= | < | >
//	literal   := "quoted string" | number | true | false | bareword
//
// varpath is a dotted key resolved against the variable bag. Comparisons are
// numeric when both sides parse as numbers, else string. Keeping it this small
// avoids smuggling a scripting language into the runtime before the contract
// for one exists.

var compareOps = []string{"==", "!=", "<=", ">=", "<", ">"}

type varGetter func(string) (any, bool)

// evalCondition evaluates a boolean condition. An empty expression is false
// (a branch node with no condition is a validation error, not a runtime panic).
func evalCondition(expr string, get varGetter) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false, nil
	}
	for _, op := range compareOps {
		if i := strings.Index(expr, op); i >= 0 {
			lhs := strings.TrimSpace(expr[:i])
			rhs := strings.TrimSpace(expr[i+len(op):])
			return compare(resolve(lhs, get), parseLiteral(rhs), op)
		}
	}
	// Bare var path → truthy test.
	v, ok := get(expr)
	return ok && truthy(v), nil
}

// evalValue resolves a var path (or literal) to its string form, for
// switch_case matching.
func evalValue(expr string, get varGetter) string {
	return toStr(resolve(strings.TrimSpace(expr), get))
}

// resolve returns the value of a var path, or the path itself parsed as a
// literal when it is quoted/numeric/bool (so `"gold"` and 3 work as operands).
func resolve(token string, get varGetter) any {
	if isLiteral(token) {
		return parseLiteral(token)
	}
	if v, ok := get(token); ok {
		return v
	}
	return nil
}

func isLiteral(t string) bool {
	if t == "true" || t == "false" {
		return true
	}
	if len(t) >= 2 && (t[0] == '"' || t[0] == '\'') {
		return true
	}
	_, err := strconv.ParseFloat(t, 64)
	return err == nil
}

func parseLiteral(t string) any {
	t = strings.TrimSpace(t)
	switch t {
	case "true":
		return true
	case "false":
		return false
	}
	if len(t) >= 2 && (t[0] == '"' || t[0] == '\'') {
		return strings.Trim(t, `"'`)
	}
	if f, err := strconv.ParseFloat(t, 64); err == nil {
		return f
	}
	return t
}

func compare(lhs, rhs any, op string) (bool, error) {
	lf, lok := toFloat(lhs)
	rf, rok := toFloat(rhs)
	if lok && rok {
		switch op {
		case "==":
			return lf == rf, nil
		case "!=":
			return lf != rf, nil
		case "<":
			return lf < rf, nil
		case ">":
			return lf > rf, nil
		case "<=":
			return lf <= rf, nil
		case ">=":
			return lf >= rf, nil
		}
	}
	ls, rs := toStr(lhs), toStr(rhs)
	switch op {
	case "==":
		return ls == rs, nil
	case "!=":
		return ls != rs, nil
	case "<", ">", "<=", ">=":
		return false, fmt.Errorf("runtime: operator %q needs numeric operands (got %q, %q)", op, ls, rs)
	}
	return false, fmt.Errorf("runtime: unknown operator %q", op)
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case int:
		return x != 0
	default:
		return true
	}
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func toStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}
