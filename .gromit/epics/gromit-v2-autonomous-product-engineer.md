---
epic_id: gromit-v2-autonomous-product-engineer
created: 2026-03-04
---

# Gromit v2: The Autonomous Product Engineer

## Problem
The current Gromit loop (v1) is a monolithic, bead-centric orchestrator that has become fragile and difficult to debug. Human intervention is frequently required for merge conflicts, technical reviews, and low-level task setup, which contradicts the core vision of "Rare Human Intervention" (<10%).

## Vision
Transform Gromit into a **Goal-Oriented Autonomous Engine** that manages the entire lifecycle of a feature (Spec/Epic) with minimal human tactical intervention. The human defines "Intent" (via `refine`), and the loop handles the "How" (Planning, Decomposition, Implementation, Review, and Integration).

## Scope
- **Goal-Oriented Orchestration:** A new v2 runner that iterates on Specs, not just Beads.
- **Immutable Pipeline:** Commit-per-iteration and snapshotting for time-travel debugging.
- **Recursive Quality:** Mandatory self-review and Andon-style escalation (L1-L4).
- **Integration Coordinator:** Single-writer main integration via an automated queue.
- **Minimalist Interface:** The human boundary is centered on Spec approval and strategic intervention.

## Key Outcomes
1. **Zero Merge Conflicts:** Automated rebase/merge/push managed by the loop.
2. **Self-Correcting:** The loop reviews its own work against `RULES.md`.
3. **Autonomous Setup:** Automated Planning and Decomposition as standard stages.
4. **Observable:** Time-travel debugging through iteration snapshots.
