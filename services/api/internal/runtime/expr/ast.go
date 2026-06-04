package expr

// Node is an expression AST node.
type Node interface{ isNode() }

// VarNode is a (possibly dotted) variable path: customer.tier, items.0.price.
type VarNode struct{ Path string }

// LitNode is a string | float64 | bool literal.
type LitNode struct{ Val any }

// UnaryNode is NOT x (the only unary operator — no arithmetic negation).
type UnaryNode struct {
	Op string // "NOT"
	X  Node
}

// BinaryNode is a boolean (AND/OR) or comparison (== != < <= > >=) node.
type BinaryNode struct {
	Op   string
	L, R Node
}

// CallNode is a namespaced function call: str.upper(x), num.abs(score).
type CallNode struct {
	Ns, Name string
	Args     []Node
}

func (VarNode) isNode()    {}
func (LitNode) isNode()    {}
func (UnaryNode) isNode()  {}
func (BinaryNode) isNode() {}
func (CallNode) isNode()   {}
