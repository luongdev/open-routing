import { describe, it, expect } from 'vitest';
import { computeVarBag, type TraceStep } from './playground-mock-data.js';

function step(partial: Partial<TraceStep> & Pick<TraceStep, 'id' | 'node_kind' | 'outputs'>): TraceStep {
  return {
    node_id: partial.id,
    label: partial.node_kind,
    started_at_ms: 0,
    duration_ms: 0,
    status: 'ok',
    inputs: {},
    ...partial,
  } as TraceStep;
}

describe('computeVarBag', () => {
  it('seeds the bag from the LIVE init vars passed in, not the hardcoded mock', () => {
    const initVars = [
      { key: 'customer.tier', value: 'gold111', type: 'string' as const, source: 'user' as const },
    ];
    const bag = computeVarBag(-1, [], initVars);
    expect(bag.find(e => e.key === 'customer.tier')?.value).toBe('gold111');
  });

  it('a capture node writes its `captured` value into save_as (not an empty var)', () => {
    const trace = [step({ id: 's2', node_kind: 'prompt_text', outputs: { captured: 'alo??', port: 'captured', save_as: 'answer' } })];
    const bag = computeVarBag(0, trace);
    expect(bag.find(e => e.key === 'answer')?.value).toBe('alo??');
  });

  it('set_var contributes ONE entry (var=value), not name+value rows', () => {
    const trace = [step({ id: 's1', node_kind: 'set_var', outputs: { name: 'var_a', value: 1 } })];
    const bag = computeVarBag(0, trace);
    const keys = bag.map(e => e.key);
    expect(keys).toContain('var_a');
    expect(keys).not.toContain('name');
    expect(keys).not.toContain('value');
    expect(bag.find(e => e.key === 'var_a')?.value).toBe(1);
  });

  it('compute maps {var,result} to a single entry; bare compute (no var) writes nothing', () => {
    const trace = [
      step({ id: 's1', node_kind: 'compute', outputs: { var: 'score', result: 5 } }),
      step({ id: 's2', node_kind: 'compute', outputs: { result: 9 } }),
    ];
    const bag = computeVarBag(1, trace);
    expect(bag.find(e => e.key === 'score')?.value).toBe(5);
    expect(bag.map(e => e.key)).not.toContain('result');
  });

  it('http_request save_as surfaces the captured response as one variable', () => {
    const resp = { items: [{ name: 'Alice' }], page: { total: 1 } };
    const trace = [
      step({ id: 's1', node_kind: 'http_request', label: 'HTTP request', outputs: { node: 'http_request', mode: 'recorded', method: 'GET', save_as: 'var_b', response: resp } }),
    ];
    const bag = computeVarBag(0, trace);
    const entry = bag.find(e => e.key === 'var_b');
    expect(entry?.value).toEqual(resp);
    // the raw record metadata is NOT leaked as variables
    expect(bag.map(e => e.key)).not.toContain('response');
    expect(bag.map(e => e.key)).not.toContain('save_as');
  });

  it('non-writing nodes (match_skill) do not leak their outputs as variables', () => {
    const trace = [
      step({ id: 's1', node_kind: 'match_skill', outputs: { candidates: 0, min_proficiency: 2, skill: 'sua_tieng_cho' } }),
    ];
    const bag = computeVarBag(0, trace);
    const keys = bag.map(e => e.key);
    expect(keys).not.toContain('candidates');
    expect(keys).not.toContain('skill');
    expect(keys).not.toContain('min_proficiency');
  });
});
