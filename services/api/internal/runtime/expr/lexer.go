package expr

import (
	"fmt"
	"strings"
)

// ParseError is a positioned syntax error.
type ParseError struct {
	Pos int
	Msg string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("expression syntax error at %d: %s", e.Pos, e.Msg)
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// lex tokenizes src. Identifiers may contain dots (dotted var paths and ns.name
// calls; the parser decides call vs var by a following "("). Negative number
// literals are lexed when "-" is immediately followed by a digit.
func lex(src string) ([]token, *ParseError) {
	var toks []token
	i, n := 0, len(src)
	for i < n {
		c := src[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		start := i
		switch {
		case isIdentStart(c):
			j := i + 1
			for j < n && isIdentPart(src[j]) {
				j++
			}
			word := src[i:j]
			switch strings.ToLower(word) {
			case "and":
				toks = append(toks, token{tkAnd, word, start})
			case "or":
				toks = append(toks, token{tkOr, word, start})
			case "not":
				toks = append(toks, token{tkNot, word, start})
			case "true", "false":
				toks = append(toks, token{tkBool, strings.ToLower(word), start})
			default:
				toks = append(toks, token{tkIdent, word, start})
			}
			i = j
		case isDigit(c) || (c == '-' && i+1 < n && isDigit(src[i+1])):
			j := i + 1
			for j < n && (isDigit(src[j]) || src[j] == '.') {
				j++
			}
			toks = append(toks, token{tkNumber, src[i:j], start})
			i = j
		case c == '"' || c == '\'':
			s, j, err := lexString(src, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, token{tkString, s, start})
			i = j
		case c == '(':
			toks = append(toks, token{tkLParen, "(", start})
			i++
		case c == ')':
			toks = append(toks, token{tkRParen, ")", start})
			i++
		case c == ',':
			toks = append(toks, token{tkComma, ",", start})
			i++
		case c == '=' || c == '!' || c == '<' || c == '>':
			if op, ok := lexOp(src, i); ok {
				toks = append(toks, token{tkOp, op, start})
				i += len(op)
			} else {
				return nil, &ParseError{i, fmt.Sprintf("unexpected %q", string(c))}
			}
		default:
			return nil, &ParseError{i, fmt.Sprintf("unexpected character %q", string(c))}
		}
	}
	toks = append(toks, token{tkEOF, "", n})
	return toks, nil
}

func lexOp(src string, i int) (string, bool) {
	if i+1 < len(src) {
		two := src[i : i+2]
		switch two {
		case "==", "!=", "<=", ">=":
			return two, true
		}
	}
	switch src[i] {
	case '<', '>':
		return string(src[i]), true
	}
	return "", false // lone '=' or '!' is invalid
}

// lexString reads a quoted string starting at i; supports \\ and \<quote>.
func lexString(src string, i int) (string, int, *ParseError) {
	quote := src[i]
	var b strings.Builder
	j := i + 1
	for j < len(src) {
		c := src[j]
		if c == '\\' && j+1 < len(src) {
			nc := src[j+1]
			if nc == quote || nc == '\\' {
				b.WriteByte(nc)
				j += 2
				continue
			}
		}
		if c == quote {
			return b.String(), j + 1, nil
		}
		b.WriteByte(c)
		j++
	}
	return "", 0, &ParseError{i, "unterminated string literal"}
}
