// Pure layered top-down auto-layout for the flow canvas. A node's layer is the
// LONGEST path from a root (trigger / parent-less node) so it sits below ALL of
// its parents in a multi-parent DAG. Computed by memoized DFS over reverse
// edges with a recursion-stack guard, so a back-edge in a cycle is ignored and
// the pass stays O(V+E) (general-graph longest path would be NP-hard).
import type { FlowNode, FlowEdge } from '../shell/playground-mock-data.js';

export interface NodePos {
  id: string;
  x: number;
  y: number;
}

const V_GAP = 180; // vertical distance between layers
const H_GAP = 220; // horizontal distance between siblings in a layer
const ORIGIN_X = 80;
const ORIGIN_Y = 60;

export function autoArrange(nodes: FlowNode[], edges: FlowEdge[]): NodePos[] {
  if (nodes.length === 0) return [];
  const ids = new Set(nodes.map(n => n.id));
  const parents = new Map<string, string[]>();
  for (const e of edges) {
    if (!ids.has(e.from) || !ids.has(e.to)) continue;
    (parents.get(e.to) ?? parents.set(e.to, []).get(e.to)!).push(e.from);
  }

  const memo = new Map<string, number>();
  const onStack = new Set<string>();
  const layerOf = (id: string): number => {
    const cached = memo.get(id);
    if (cached !== undefined) return cached;
    onStack.add(id);
    let best = 0;
    for (const p of parents.get(id) ?? []) {
      if (onStack.has(p)) continue; // back-edge in a cycle — don't recurse
      best = Math.max(best, layerOf(p) + 1);
    }
    onStack.delete(id);
    memo.set(id, best);
    return best;
  };

  // Group node ids by layer, preserving first-seen order within a layer.
  const byLayer = new Map<number, string[]>();
  for (const n of nodes) {
    const l = layerOf(n.id);
    (byLayer.get(l) ?? byLayer.set(l, []).get(l)!).push(n.id);
  }

  const pos = new Map<string, NodePos>();
  for (const [layer, row] of byLayer) {
    row.forEach((id, i) => {
      pos.set(id, { id, x: ORIGIN_X + i * H_GAP, y: ORIGIN_Y + layer * V_GAP });
    });
  }
  return nodes.map(n => pos.get(n.id)!);
}
