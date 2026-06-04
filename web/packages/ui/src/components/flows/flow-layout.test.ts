import { describe, it, expect } from 'vitest';
import { autoArrange } from './flow-layout.js';
import type { FlowNode, FlowEdge } from '../shell/playground-mock-data.js';

function node(id: string, kind = 'log'): FlowNode {
  return { id, kind: kind as FlowNode['kind'], label: id, description: '', x: 0, y: 0 };
}

describe('autoArrange', () => {
  it('layers nodes top-down by longest path from the trigger', () => {
    const nodes = [node('t', 'trigger'), node('a'), node('b'), node('end', 'end')];
    const edges: FlowEdge[] = [
      { id: 'e1', from: 't', to: 'a' },
      { id: 'e2', from: 'a', to: 'b' },
      { id: 'e3', from: 'b', to: 'end' },
    ];
    const pos = new Map(autoArrange(nodes, edges).map(p => [p.id, p]));
    // Strictly increasing y down the chain.
    expect(pos.get('t')!.y).toBeLessThan(pos.get('a')!.y);
    expect(pos.get('a')!.y).toBeLessThan(pos.get('b')!.y);
    expect(pos.get('b')!.y).toBeLessThan(pos.get('end')!.y);
  });

  it('places a multi-parent node below ALL its parents (longest path, not shortest)', () => {
    // t -> a -> c, and t -> c directly. c must sit below a (longest path = 2),
    // not at layer 1 (shortest path).
    const nodes = [node('t', 'trigger'), node('a'), node('c')];
    const edges: FlowEdge[] = [
      { id: 'e1', from: 't', to: 'a' },
      { id: 'e2', from: 'a', to: 'c' },
      { id: 'e3', from: 't', to: 'c' },
    ];
    const pos = new Map(autoArrange(nodes, edges).map(p => [p.id, p]));
    expect(pos.get('c')!.y).toBeGreaterThan(pos.get('a')!.y);
  });

  it('terminates on a cycle (back-edge ignored)', () => {
    const nodes = [node('t', 'trigger'), node('a'), node('b')];
    const edges: FlowEdge[] = [
      { id: 'e1', from: 't', to: 'a' },
      { id: 'e2', from: 'a', to: 'b' },
      { id: 'e3', from: 'b', to: 'a' }, // cycle
    ];
    const pos = autoArrange(nodes, edges); // must not hang
    expect(pos).toHaveLength(3);
  });

  it('returns a position for every node, including disconnected ones', () => {
    const nodes = [node('t', 'trigger'), node('island')];
    const pos = autoArrange(nodes, []);
    expect(pos.map(p => p.id).sort()).toEqual(['island', 't']);
    expect(pos.every(p => Number.isFinite(p.x) && Number.isFinite(p.y))).toBe(true);
  });
});
