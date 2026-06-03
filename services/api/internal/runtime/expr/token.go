package expr

type tokKind int

const (
	tkEOF tokKind = iota
	tkIdent
	tkNumber
	tkString
	tkBool
	tkAnd
	tkOr
	tkNot
	tkOp // == != <= >= < >
	tkLParen
	tkRParen
	tkComma
)

type token struct {
	kind tokKind
	lit  string // identifier text, raw string value, number text, op, or "true"/"false"
	pos  int    // byte offset in source, for error messages
}
