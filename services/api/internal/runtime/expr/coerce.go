// Package expr is the v0.2 routing-condition expression engine: a real
// lexer/parser/evaluator over boolean logic (AND/OR/NOT), comparisons,
// dotted variable paths, literals, and namespaced function calls (str.*,
// num.*, arr.*, logic.*, date.*). Math is via num.* functions — there is NO
// infix arithmetic. It is deterministic: it reads only its Env (the variable
// bag + Now()) and pure functions, so simulation/replay reproduce exactly.
package expr

import (
	"fmt"
	"strconv"
)

// unresolved is an identifier that didn't resolve to a variable. In a boolean
// context it is falsey (a bare `missing` var → false); in a comparison it acts
// as a string literal of the identifier text (so `tier == gold` compares the
// `tier` value against the literal "gold"). This preserves the original
// "bareword = variable if defined, else literal string" ergonomics.
type unresolved string

// truthy coerces a value to a boolean for AND/OR/NOT and bare-variable tests.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case unresolved:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case int:
		return x != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
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
	case unresolved:
		return string(x)
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

// compare evaluates lhs OP rhs: numeric when both coerce to float, else string
// equality; ordering (< <= > >=) on non-numeric operands is an error.
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
		return false, fmt.Errorf("operator %q needs numeric operands (got %q, %q)", op, ls, rs)
	}
	return false, fmt.Errorf("unknown operator %q", op)
}
