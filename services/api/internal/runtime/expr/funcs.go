package expr

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Fn is one registered function. Adding a function = one register() call.
// Call receives `now` (the deterministic clock, for date.*) + already-evaluated
// args (functions are EAGER — args are evaluated before the call).
type Fn struct {
	Ns        string                                       `json:"ns"`
	Name      string                                       `json:"name"`
	Arity     int                                          `json:"arity"` // -1 = variadic
	Summary   string                                       `json:"summary"`
	Signature string                                       `json:"signature"`
	Call      func(now time.Time, args []any) (any, error) `json:"-"`
}

var registry = map[string]Fn{}

func register(f Fn) {
	key := f.Ns + "." + f.Name
	if _, dup := registry[key]; dup {
		panic("expr: duplicate function " + key)
	}
	registry[key] = f
}

// Lookup returns the function for a namespace+name.
func Lookup(ns, name string) (Fn, bool) {
	f, ok := registry[ns+"."+name]
	return f, ok
}

// Catalog returns every function sorted by ns then name — the single source the
// UI autocomplete consumes via GET /v1/meta/expr-functions.
func Catalog() []Fn {
	out := make([]Fn, 0, len(registry))
	for _, f := range registry {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ns != out[j].Ns {
			return out[i].Ns < out[j].Ns
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ---- arg coercion helpers ----

func argStr(args []any, i int) string { return toStr(args[i]) }

func argFloat(args []any, i int) (float64, error) {
	f, ok := toFloat(args[i])
	if !ok {
		return 0, fmt.Errorf("argument %d must be a number, got %T", i+1, args[i])
	}
	return f, nil
}

func argArr(args []any, i int) ([]any, error) {
	a, ok := args[i].([]any)
	if !ok {
		return nil, fmt.Errorf("argument %d must be an array, got %T", i+1, args[i])
	}
	return a, nil
}

func init() {
	// ---- str.* ----
	register(Fn{"str", "upper", 1, "Uppercase", "str.upper(s) -> string", func(_ time.Time, a []any) (any, error) { return strings.ToUpper(argStr(a, 0)), nil }})
	register(Fn{"str", "lower", 1, "Lowercase", "str.lower(s) -> string", func(_ time.Time, a []any) (any, error) { return strings.ToLower(argStr(a, 0)), nil }})
	register(Fn{"str", "trim", 1, "Trim whitespace", "str.trim(s) -> string", func(_ time.Time, a []any) (any, error) { return strings.TrimSpace(argStr(a, 0)), nil }})
	register(Fn{"str", "len", 1, "Length (runes)", "str.len(s) -> number", func(_ time.Time, a []any) (any, error) { return float64(utf8.RuneCountInString(argStr(a, 0))), nil }})
	register(Fn{"str", "contains", 2, "Substring check", "str.contains(s, sub) -> bool", func(_ time.Time, a []any) (any, error) { return strings.Contains(argStr(a, 0), argStr(a, 1)), nil }})
	register(Fn{"str", "startsWith", 2, "Prefix check", "str.startsWith(s, p) -> bool", func(_ time.Time, a []any) (any, error) { return strings.HasPrefix(argStr(a, 0), argStr(a, 1)), nil }})
	register(Fn{"str", "endsWith", 2, "Suffix check", "str.endsWith(s, p) -> bool", func(_ time.Time, a []any) (any, error) { return strings.HasSuffix(argStr(a, 0), argStr(a, 1)), nil }})
	register(Fn{"str", "replace", 3, "Replace all", "str.replace(s, old, new) -> string", func(_ time.Time, a []any) (any, error) {
		return strings.ReplaceAll(argStr(a, 0), argStr(a, 1), argStr(a, 2)), nil
	}})
	register(Fn{"str", "split", 2, "Split by separator", "str.split(s, sep) -> array", func(_ time.Time, a []any) (any, error) {
		parts := strings.Split(argStr(a, 0), argStr(a, 1))
		out := make([]any, len(parts))
		for i, p := range parts {
			out[i] = p
		}
		return out, nil
	}})
	register(Fn{"str", "join", 2, "Join array with separator", "str.join(arr, sep) -> string", func(_ time.Time, a []any) (any, error) {
		arr, err := argArr(a, 0)
		if err != nil {
			return nil, err
		}
		parts := make([]string, len(arr))
		for i, v := range arr {
			parts[i] = toStr(v)
		}
		return strings.Join(parts, argStr(a, 1)), nil
	}})
	register(Fn{"str", "substring", 3, "Substring [start,end)", "str.substring(s, start, end) -> string", func(_ time.Time, a []any) (any, error) {
		r := []rune(argStr(a, 0))
		start, err := argFloat(a, 1)
		if err != nil {
			return nil, err
		}
		end, err := argFloat(a, 2)
		if err != nil {
			return nil, err
		}
		s, e := clampIdx(int(start), len(r)), clampIdx(int(end), len(r))
		if s > e {
			s = e
		}
		return string(r[s:e]), nil
	}})
	register(Fn{"str", "padLeft", 3, "Pad left to length", "str.padLeft(s, len, pad) -> string", func(_ time.Time, a []any) (any, error) { return pad(a, true) }})
	register(Fn{"str", "padRight", 3, "Pad right to length", "str.padRight(s, len, pad) -> string", func(_ time.Time, a []any) (any, error) { return pad(a, false) }})

	// ---- num.* ----
	register(Fn{"num", "abs", 1, "Absolute value", "num.abs(x) -> number", num1(math.Abs)})
	register(Fn{"num", "sqrt", 1, "Square root", "num.sqrt(x) -> number", num1(math.Sqrt)})
	register(Fn{"num", "round", 1, "Round to nearest", "num.round(x) -> number", num1(math.Round)})
	register(Fn{"num", "floor", 1, "Round down", "num.floor(x) -> number", num1(math.Floor)})
	register(Fn{"num", "ceil", 1, "Round up", "num.ceil(x) -> number", num1(math.Ceil)})
	register(Fn{"num", "mod", 2, "Modulo", "num.mod(a, b) -> number", num2(math.Mod)})
	register(Fn{"num", "pow", 2, "Power", "num.pow(a, b) -> number", num2(math.Pow)})
	register(Fn{"num", "min", -1, "Minimum (variadic)", "num.min(...) -> number", numReduce(math.Min)})
	register(Fn{"num", "max", -1, "Maximum (variadic)", "num.max(...) -> number", numReduce(math.Max)})
	register(Fn{"num", "sum", 1, "Sum of array", "num.sum(arr) -> number", numArr(func(xs []float64) float64 {
		s := 0.0
		for _, x := range xs {
			s += x
		}
		return s
	})})
	register(Fn{"num", "avg", 1, "Average of array", "num.avg(arr) -> number", numArr(func(xs []float64) float64 {
		if len(xs) == 0 {
			return 0
		}
		s := 0.0
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	})})
	register(Fn{"num", "parseInt", 1, "Parse integer from string", "num.parseInt(s) -> number", func(_ time.Time, a []any) (any, error) {
		i, err := strconv.Atoi(strings.TrimSpace(argStr(a, 0)))
		if err != nil {
			return nil, fmt.Errorf("num.parseInt: %q is not an integer", argStr(a, 0))
		}
		return float64(i), nil
	}})
	register(Fn{"num", "parseFloat", 1, "Parse float from string", "num.parseFloat(s) -> number", func(_ time.Time, a []any) (any, error) {
		f, err := strconv.ParseFloat(strings.TrimSpace(argStr(a, 0)), 64)
		if err != nil {
			return nil, fmt.Errorf("num.parseFloat: %q is not a number", argStr(a, 0))
		}
		return f, nil
	}})

	// ---- arr.* ----
	register(Fn{"arr", "len", 1, "Array length", "arr.len(arr) -> number", func(_ time.Time, a []any) (any, error) {
		arr, err := argArr(a, 0)
		if err != nil {
			return nil, err
		}
		return float64(len(arr)), nil
	}})
	register(Fn{"arr", "contains", 2, "Array membership", "arr.contains(arr, v) -> bool", func(_ time.Time, a []any) (any, error) {
		arr, err := argArr(a, 0)
		if err != nil {
			return nil, err
		}
		for _, v := range arr {
			if eq, _ := compare(v, a[1], "=="); eq {
				return true, nil
			}
		}
		return false, nil
	}})
	register(Fn{"arr", "first", 1, "First element", "arr.first(arr) -> any", func(_ time.Time, a []any) (any, error) { return arrAt(a, 0, 0) }})
	register(Fn{"arr", "last", 1, "Last element", "arr.last(arr) -> any", func(_ time.Time, a []any) (any, error) { return arrAt(a, 0, -1) }})

	// ---- logic.* (EAGER: args evaluated before the call) ----
	register(Fn{"logic", "coalesce", -1, "First non-empty value", "logic.coalesce(...) -> any", func(_ time.Time, a []any) (any, error) {
		for _, v := range a {
			if truthy(v) {
				return v, nil
			}
		}
		return nil, nil
	}})
	register(Fn{"logic", "ifElse", 3, "cond ? a : b (eager)", "logic.ifElse(cond, a, b) -> any", func(_ time.Time, a []any) (any, error) {
		if truthy(a[0]) {
			return a[1], nil
		}
		return a[2], nil
	}})

	// ---- date.* (UTC, deterministic; dates are unix-seconds numbers) ----
	register(Fn{"date", "now", 0, "Current time (unix seconds, UTC)", "date.now() -> number", func(now time.Time, _ []any) (any, error) { return float64(now.UTC().Unix()), nil }})
	register(Fn{"date", "addDays", 2, "Add days to a unix-seconds date", "date.addDays(ts, n) -> number", func(_ time.Time, a []any) (any, error) {
		ts, err := argFloat(a, 0)
		if err != nil {
			return nil, err
		}
		n, err := argFloat(a, 1)
		if err != nil {
			return nil, err
		}
		return ts + n*86400, nil
	}})
	register(Fn{"date", "year", 1, "Year of a unix-seconds date (UTC)", "date.year(ts) -> number", dateField(func(t time.Time) int { return t.Year() })})
	register(Fn{"date", "month", 1, "Month 1-12 (UTC)", "date.month(ts) -> number", dateField(func(t time.Time) int { return int(t.Month()) })})
	register(Fn{"date", "day", 1, "Day of month (UTC)", "date.day(ts) -> number", dateField(func(t time.Time) int { return t.Day() })})
}

// ---- small function builders ----

func num1(f func(float64) float64) func(time.Time, []any) (any, error) {
	return func(_ time.Time, a []any) (any, error) {
		x, err := argFloat(a, 0)
		if err != nil {
			return nil, err
		}
		return f(x), nil
	}
}

func num2(f func(float64, float64) float64) func(time.Time, []any) (any, error) {
	return func(_ time.Time, a []any) (any, error) {
		x, err := argFloat(a, 0)
		if err != nil {
			return nil, err
		}
		y, err := argFloat(a, 1)
		if err != nil {
			return nil, err
		}
		return f(x, y), nil
	}
}

func numReduce(f func(float64, float64) float64) func(time.Time, []any) (any, error) {
	return func(_ time.Time, a []any) (any, error) {
		if len(a) == 0 {
			return nil, fmt.Errorf("requires at least one argument")
		}
		acc, err := argFloat(a, 0)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(a); i++ {
			x, err := argFloat(a, i)
			if err != nil {
				return nil, err
			}
			acc = f(acc, x)
		}
		return acc, nil
	}
}

func numArr(f func([]float64) float64) func(time.Time, []any) (any, error) {
	return func(_ time.Time, a []any) (any, error) {
		arr, err := argArr(a, 0)
		if err != nil {
			return nil, err
		}
		xs := make([]float64, len(arr))
		for i, v := range arr {
			x, ok := toFloat(v)
			if !ok {
				return nil, fmt.Errorf("array element %d is not a number", i)
			}
			xs[i] = x
		}
		return f(xs), nil
	}
}

func dateField(f func(time.Time) int) func(time.Time, []any) (any, error) {
	return func(_ time.Time, a []any) (any, error) {
		ts, err := argFloat(a, 0)
		if err != nil {
			return nil, err
		}
		return float64(f(time.Unix(int64(ts), 0).UTC())), nil
	}
}

func arrAt(a []any, i, idx int) (any, error) {
	arr, err := argArr(a, i)
	if err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, nil
	}
	if idx < 0 {
		idx = len(arr) + idx
	}
	if idx < 0 || idx >= len(arr) {
		return nil, nil
	}
	return arr[idx], nil
}

func clampIdx(i, n int) int {
	if i < 0 {
		return 0
	}
	if i > n {
		return n
	}
	return i
}

// maxPadWidth bounds str.padLeft/Right so a hostile width (e.g. 1e9, or NaN/Inf
// from int() conversion) can't spin the pad loop into a CPU/memory DoS during
// flow execution (cross-AI review MED).
const maxPadWidth = 4096

func pad(a []any, left bool) (any, error) {
	s := argStr(a, 0)
	width, err := argFloat(a, 1)
	if err != nil {
		return nil, err
	}
	if math.IsNaN(width) || width > maxPadWidth {
		return nil, fmt.Errorf("pad width must be a finite number <= %d", maxPadWidth)
	}
	padStr := argStr(a, 2)
	if padStr == "" {
		padStr = " "
	}
	for utf8.RuneCountInString(s) < int(width) {
		if left {
			s = padStr + s
		} else {
			s = s + padStr
		}
	}
	return s, nil
}
