---
sketch: 001
name: embed-navigation
question: "How does navigation work in an embedded constrained space?"
winner: "B"
tags: [embed, navigation, layout]
---

# Sketch 001: Embed Navigation

## Design Question
How does the embed handle navigation between catalog entities without conflicting with the host application's own layout?

## How to View
open .planning/sketches/001-embed-navigation/index.html

## Variants
- **A: Top Nav (Tabs)** — Uses horizontal space, feels like a secondary nav layer under the host app's header.
- **B: Thin Sidebar** — Icon-only vertical sidebar. Saves vertical space, feels like a discrete "app within an app".
- **C: Dropdown Selector** — Maximizes space by placing navigation into a header dropdown. Good for extremely constrained widths.

## What to Look For
- Which pattern feels most "embeddable" regardless of what the host application looks like?
- How much space does the navigation consume?
- Is it clear that the navigation controls the catalog, not the host app?
