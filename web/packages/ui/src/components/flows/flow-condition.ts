// Visual AND/OR group model for the flow condition builder, and lossless
// serialization to the backend DSL. The DSL string is the source of truth; this
// model is an ephemeral projection. dslToGroup parses only the "simple" subset
// (groups of comparisons/truthy joined by AND/OR/NOT) and returns null for
// anything function-using or non-round-trippable — the caller then stays in the
// raw Advanced editor.

export type CondOp = '==' | '!=' | '<' | '<=' | '>' | '>=';
export const COND_OPS: CondOp[] = ['==', '!=', '<', '<=', '>', '>='];

export interface Comparison {
  kind: 'cmp';
  mode: 'cmp' | 'truthy';
  lhs: string;
  op: CondOp;
  rhs: string;
}
export interface Group {
  kind: 'group';
  op: 'AND' | 'OR';
  not?: boolean;
  children: CondNode[];
}
export type CondNode = Comparison | Group;

let _seq = 0;
export function newComparison(): Comparison {
  return { kind: 'cmp', mode: 'cmp', lhs: '', op: '==', rhs: '' };
}
export function newGroup(op: 'AND' | 'OR' = 'AND'): Group {
  return { kind: 'group', op, children: [] };
}
// Stable-ish id for Lit's repeat() keying within a render pass.
export function condKey(): string {
  return 'c' + (++_seq).toString(36);
}

// ---- serialize: Group → DSL ----

function quoteIfNeeded(rhs: string): string {
  const s = rhs.trim();
  if (s === '') return '""';
  if (s === 'true' || s === 'false') return s;
  if (/^-?\d+(\.\d+)?$/.test(s)) return s; // number
  if (/^[A-Za-z_][A-Za-z0-9_.]*$/.test(s)) return s; // bareword (var or unquoted string)
  return '"' + s.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"';
}

function leafToDsl(c: Comparison): string {
  const lhs = c.lhs.trim();
  if (!lhs) return '';
  if (c.mode === 'truthy') return lhs;
  return `${lhs} ${c.op} ${quoteIfNeeded(c.rhs)}`;
}

export function groupToDsl(g: Group): string {
  const parts = g.children
    .map(ch => (ch.kind === 'group' ? wrap(ch) : leafToDsl(ch)))
    .filter(s => s !== '');
  const joined = parts.join(` ${g.op} `);
  if (g.not) return joined === '' ? '' : `NOT (${joined})`;
  return joined;
}

function wrap(g: Group): string {
  const inner = groupToDsl(g);
  if (inner === '') return '';
  return g.not ? inner : `(${inner})`; // groupToDsl already wraps a NOT group
}

// ---- parse: DSL → Group | null (simple subset only) ----

const OP_RE = /(<=|>=|==|!=|<|>)/;

export function dslToGroup(dsl: string): Group | null {
  const s = dsl.trim();
  if (s === '') return newGroup('AND');
  if (/[A-Za-z_][A-Za-z0-9_]*\.[A-Za-z0-9_]+\s*\(/.test(s)) return null; // namespaced fn → Advanced
  try {
    const node = parseOr(new Cursor(s));
    // Top level must be a Group; wrap a lone comparison in an AND group.
    const g = node.kind === 'group' ? node : { kind: 'group' as const, op: 'AND' as const, children: [node] };
    return g;
  } catch {
    return null;
  }
}

class Cursor {
  s: string;
  i = 0;
  constructor(s: string) { this.s = s; }
  ws() { while (this.i < this.s.length && /\s/.test(this.s[this.i]!)) this.i++; }
  eof() { this.ws(); return this.i >= this.s.length; }
  // Peek a keyword (AND/OR/NOT) without consuming.
  peekKw(kw: string): boolean {
    this.ws();
    const seg = this.s.slice(this.i, this.i + kw.length);
    if (seg.toUpperCase() !== kw) return false;
    const after = this.s[this.i + kw.length];
    return after === undefined || /\s|\(/.test(after);
  }
  takeKw(kw: string) { this.i += kw.length; }
}

function parseOr(c: Cursor): CondNode {
  let left = parseAnd(c);
  const children: CondNode[] = [left];
  while (c.peekKw('OR')) {
    c.takeKw('OR');
    children.push(parseAnd(c));
  }
  if (children.length === 1) return left;
  return { kind: 'group', op: 'OR', children: children.map(flattenSameOp('OR')).flat() };
}

function parseAnd(c: Cursor): CondNode {
  const children: CondNode[] = [parseUnary(c)];
  while (c.peekKw('AND')) {
    c.takeKw('AND');
    children.push(parseUnary(c));
  }
  if (children.length === 1) return children[0]!;
  return { kind: 'group', op: 'AND', children: children.map(flattenSameOp('AND')).flat() };
}

// Flatten a nested same-op group so `a AND b AND c` is one AND group, not nested.
function flattenSameOp(op: 'AND' | 'OR') {
  return (n: CondNode): CondNode[] => {
    if (n.kind === 'group' && n.op === op && !n.not) return n.children;
    return [n];
  };
}

function parseUnary(c: Cursor): CondNode {
  if (c.peekKw('NOT')) {
    c.takeKw('NOT');
    const inner = parseUnary(c);
    if (inner.kind === 'group') return { ...inner, not: true };
    return { kind: 'group', op: 'AND', not: true, children: [inner] };
  }
  c.ws();
  if (c.s[c.i] === '(') {
    c.i++;
    const inner = parseOr(c);
    c.ws();
    if (c.s[c.i] !== ')') throw new Error('expected )');
    c.i++;
    return inner;
  }
  return parseLeaf(c);
}

// A leaf runs until a top-level AND/OR/NOT keyword or a ) — no nested parens
// (those are handled by parseUnary). Then split on the comparison operator.
function parseLeaf(c: Cursor): Comparison {
  c.ws();
  const start = c.i;
  let depth = 0;
  while (c.i < c.s.length) {
    const ch = c.s[c.i]!;
    if (ch === '(') depth++;
    else if (ch === ')') {
      if (depth === 0) break;
      depth--;
    } else if (depth === 0 && (c.peekKw('AND') || c.peekKw('OR'))) {
      break;
    }
    if (depth === 0 && ch === '"') { // skip quoted strings
      c.i++;
      while (c.i < c.s.length && c.s[c.i] !== '"') c.i++;
    }
    c.i++;
  }
  const raw = c.s.slice(start, c.i).trim();
  if (raw === '' || /[()]/.test(raw)) throw new Error('bad leaf');
  const m = OP_RE.exec(raw);
  if (!m) {
    // bare variable → truthy test
    if (!/^[A-Za-z_][A-Za-z0-9_.]*$/.test(raw)) throw new Error('bad truthy leaf');
    return { kind: 'cmp', mode: 'truthy', lhs: raw, op: '==', rhs: '' };
  }
  const lhs = raw.slice(0, m.index).trim();
  let rhs = raw.slice(m.index + m[0].length).trim();
  if (OP_RE.test(rhs) && !/^"/.test(rhs)) throw new Error('chained comparison');
  if (/^".*"$/.test(rhs)) rhs = rhs.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, '\\');
  if (!/^[A-Za-z_][A-Za-z0-9_.]*$/.test(lhs)) throw new Error('bad lhs');
  return { kind: 'cmp', mode: 'cmp', lhs, op: m[0] as CondOp, rhs };
}
