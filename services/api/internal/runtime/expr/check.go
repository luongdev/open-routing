package expr

import "fmt"

// Check walks a parsed AST and reports the first semantic error that Parse
// cannot catch: a call to an unregistered function, or a call with the wrong
// number of arguments. It exists so the publish/validate gate rejects these
// instead of letting them surface as a runtime error mid-flow.
func Check(n Node) error {
	switch x := n.(type) {
	case UnaryNode:
		return Check(x.X)
	case BinaryNode:
		if err := Check(x.L); err != nil {
			return err
		}
		return Check(x.R)
	case CallNode:
		fn, ok := Lookup(x.Ns, x.Name)
		if !ok {
			return fmt.Errorf("unknown function %s.%s", x.Ns, x.Name)
		}
		switch {
		case fn.Arity >= 0 && len(x.Args) != fn.Arity:
			return fmt.Errorf("%s.%s expects %d argument(s), got %d", x.Ns, x.Name, fn.Arity, len(x.Args))
		// Variadic (-1): every registered variadic fn is a reduce/coalesce that
		// is meaningless with zero args (num.min(), logic.coalesce()), so the
		// gate rejects the empty call rather than deferring to a runtime error.
		case fn.Arity < 0 && len(x.Args) == 0:
			return fmt.Errorf("%s.%s requires at least one argument", x.Ns, x.Name)
		}
		for _, a := range x.Args {
			if err := Check(a); err != nil {
				return err
			}
		}
	}
	return nil
}
