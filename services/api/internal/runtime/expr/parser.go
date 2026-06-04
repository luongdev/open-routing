package expr

import (
	"strconv"
	"strings"
)

// Parse turns source into an AST. Precedence (loose→tight): OR < AND < NOT <
// comparison < primary. NOT binds looser than comparison (SQL-style:
// `NOT a == b` == `NOT (a == b)`). Comparisons are non-chaining.
// maxExprLen / maxExprDepth bound a single expression so a pathologically long
// or deeply-nested one can't DoS the parser/evaluator (review M3).
const maxExprLen = 4096
const maxExprDepth = 64

func Parse(src string) (Node, *ParseError) {
	if len(src) > maxExprLen {
		return nil, &ParseError{0, "expression too long"}
	}
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	n, perr := p.parseExpr(0)
	if perr != nil {
		return nil, perr
	}
	if p.cur().kind != tkEOF {
		return nil, &ParseError{p.cur().pos, "unexpected trailing input"}
	}
	return n, nil
}

type parser struct {
	toks  []token
	pos   int
	depth int
}

func (p *parser) cur() token  { return p.toks[p.pos] }
func (p *parser) next() token { t := p.toks[p.pos]; p.pos++; return t }

// boolBP: OR=1, AND=2; 0 = not a boolean operator.
func boolBP(k tokKind) int {
	switch k {
	case tkOr:
		return 1
	case tkAnd:
		return 2
	}
	return 0
}

func (p *parser) parseExpr(minBP int) (Node, *ParseError) {
	// Bound recursion (parens/AND/OR/NOT all re-enter here) so a deeply nested
	// expression can't overflow the Go stack (review M3).
	p.depth++
	if p.depth > maxExprDepth {
		return nil, &ParseError{p.cur().pos, "expression nested too deeply"}
	}
	defer func() { p.depth-- }()
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for {
		bp := boolBP(p.cur().kind)
		if bp == 0 || bp < minBP {
			break
		}
		opTok := p.next()
		right, err := p.parseExpr(bp + 1) // left-assoc
		if err != nil {
			return nil, err
		}
		op := "AND"
		if opTok.kind == tkOr {
			op = "OR"
		}
		left = BinaryNode{Op: op, L: left, R: right}
	}
	return left, nil
}

func (p *parser) parseNot() (Node, *ParseError) {
	if p.cur().kind == tkNot {
		p.next()
		x, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return UnaryNode{Op: "NOT", X: x}, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (Node, *ParseError) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if p.cur().kind == tkOp {
		opTok := p.next()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		if p.cur().kind == tkOp {
			return nil, &ParseError{p.cur().pos, "comparison cannot be chained — use AND/OR"}
		}
		return BinaryNode{Op: opTok.lit, L: left, R: right}, nil
	}
	return left, nil
}

func (p *parser) parsePrimary() (Node, *ParseError) {
	t := p.cur()
	switch t.kind {
	case tkLParen:
		p.next()
		n, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if p.cur().kind != tkRParen {
			return nil, &ParseError{p.cur().pos, "expected )"}
		}
		p.next()
		return n, nil
	case tkNumber:
		p.next()
		f, err := strconv.ParseFloat(t.lit, 64)
		if err != nil {
			return nil, &ParseError{t.pos, "invalid number " + t.lit}
		}
		return LitNode{Val: f}, nil
	case tkString:
		p.next()
		return LitNode{Val: t.lit}, nil
	case tkBool:
		p.next()
		return LitNode{Val: t.lit == "true"}, nil
	case tkIdent:
		p.next()
		if p.cur().kind == tkLParen {
			return p.parseCall(t)
		}
		return VarNode{Path: t.lit}, nil
	default:
		return nil, &ParseError{t.pos, "expected a value, variable, function, or ("}
	}
}

func (p *parser) parseCall(nameTok token) (Node, *ParseError) {
	p.next() // consume (
	ns, name := "", nameTok.lit
	if i := strings.Index(nameTok.lit, "."); i >= 0 {
		ns, name = nameTok.lit[:i], nameTok.lit[i+1:]
	}
	var args []Node
	if p.cur().kind != tkRParen {
		for {
			a, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			if p.cur().kind == tkComma {
				p.next()
				continue
			}
			break
		}
	}
	if p.cur().kind != tkRParen {
		return nil, &ParseError{p.cur().pos, "expected , or ) in argument list"}
	}
	p.next()
	return CallNode{Ns: ns, Name: name, Args: args}, nil
}
