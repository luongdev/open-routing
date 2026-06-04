import { describe, it, expect } from 'vitest';
import { groupToDsl, dslToGroup, type Group } from './flow-condition.js';

describe('flow-condition serialize/parse', () => {
  it('serializes a flat AND group', () => {
    const g: Group = {
      kind: 'group', op: 'AND', children: [
        { kind: 'cmp', mode: 'cmp', lhs: 'customer.tier', op: '==', rhs: 'gold' },
        { kind: 'cmp', mode: 'cmp', lhs: 'age', op: '>', rhs: '18' },
      ],
    };
    expect(groupToDsl(g)).toBe('customer.tier == gold AND age > 18');
  });

  it('serializes truthy + quotes values with spaces', () => {
    const g: Group = {
      kind: 'group', op: 'OR', children: [
        { kind: 'cmp', mode: 'truthy', lhs: 'vip', op: '==', rhs: '' },
        { kind: 'cmp', mode: 'cmp', lhs: 'name', op: '==', rhs: 'John Doe' },
      ],
    };
    expect(groupToDsl(g)).toBe('vip OR name == "John Doe"');
  });

  it('wraps a nested group and a NOT group in parens', () => {
    const g: Group = {
      kind: 'group', op: 'OR', children: [
        { kind: 'group', op: 'AND', children: [
          { kind: 'cmp', mode: 'cmp', lhs: 'a', op: '==', rhs: '1' },
          { kind: 'cmp', mode: 'cmp', lhs: 'b', op: '==', rhs: '2' },
        ] },
        { kind: 'cmp', mode: 'truthy', lhs: 'c', op: '==', rhs: '' },
      ],
    };
    expect(groupToDsl(g)).toBe('(a == 1 AND b == 2) OR c');
  });

  it('round-trips simple DSL through parse → serialize', () => {
    for (const dsl of [
      'customer.tier == gold AND age > 18',
      'vip OR name == "John Doe"',
      '(a == 1 AND b == 2) OR c',
      'NOT (a == 1 OR b == 2)',
      // A quoted value FOLLOWED BY more clauses — regression for a parser bug
      // where the leaf scan swallowed everything after the close quote.
      'name == "John Doe" AND vip',
      '(customer.region == "us-west" AND age > 18) OR vip',
    ]) {
      const g = dslToGroup(dsl);
      expect(g, dsl).not.toBeNull();
      expect(groupToDsl(g!)).toBe(dsl);
    }
  });

  it('returns null for function-using or unparseable DSL (forces Advanced)', () => {
    expect(dslToGroup('num.abs(score) > 3')).toBeNull();
    expect(dslToGroup('str.upper(name) == ALICE AND vip')).toBeNull();
    expect(dslToGroup('a < b < c')).toBeNull();
    expect(dslToGroup('vip) OR admin')).toBeNull(); // trailing tokens after a parsed prefix
    expect(dslToGroup("name == 'John Doe'")).toBeNull(); // single-quoted string not round-trippable
    expect(dslToGroup('NOT NOT vip')).toBeNull(); // double NOT collapses to one
    expect(dslToGroup('name == "a \\" b" AND vip')).toBeNull(); // escaped quote not handled
  });

  it('empty DSL → empty AND group', () => {
    const g = dslToGroup('');
    expect(g).toEqual({ kind: 'group', op: 'AND', children: [] });
  });
});
