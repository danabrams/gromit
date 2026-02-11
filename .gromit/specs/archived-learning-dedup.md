---
id: archived-learning-dedup
source_ideas: [idea-1770741412407]
created: 2026-02-11
---

# Archived Learning Dedup

## Specification

The `Add()` method in `internal/learnings/learnings.go` checks confirmed and provisional learnings for hash duplicates before adding a new learning, but does not check archived learnings. This means the same content can be archived multiple times: once via the inline filter in `Add()` (which archives generic content immediately) and again if the same content reappears later and passes through the same path.

The fix adds a hash dedup check against the archived section in `Add()`. When a new learning's hash matches an existing archived learning, `Add()` returns `(nil, nil)` — the same "silently skip" behavior used for confirmed and provisional duplicates. No save is performed, no LLM filter call is made.

This prevents duplicate entries from accumulating in the archived section and avoids wasting LLM filter calls on content that was previously evaluated and archived.

## Acceptance Criteria

- `Add()` returns `(nil, nil)` without calling the filter function when the new learning's content hash matches an existing archived learning's hash.
- `Add()` does not call `Save()` when skipping an archived duplicate.
- Existing behavior for confirmed and provisional dedup is unchanged.

## Decisions

1. **Check archived before running the filter.** The hash check is cheap (in-memory loop). Running it before the LLM filter call avoids wasting a haiku invocation on content already known to be archived.

2. **Same skip semantics as confirmed/provisional dedup.** Returning `(nil, nil)` is the established contract for "duplicate, silently skipped." No new error types or return values needed.

3. **No changes to `FilterProvisional()`.** The dedup check in `Add()` prevents duplicate content from ever reaching provisional status, so `FilterProvisional()` will never encounter it. No changes needed there.

## Research & Context

### Current State

- `Add()` at `internal/learnings/learnings.go:112-178` checks `f.confirmed` and `f.provisional` for hash duplicates (lines 119-127) but skips `f.archived`.
- `hashContent()` at `internal/learnings/learnings.go:562-569` generates 16-char hex strings (first 8 bytes of SHA256) used for dedup.
- The inline filter in `Add()` (lines 131-147) can archive a learning immediately on first add. Without the dedup check, the same content arriving a second time would trigger another filter call and create a duplicate archived entry.
- The archived section in `LEARNINGS.md` currently has 87+ entries and grows without bound. While this spec doesn't cap or purge archived entries, preventing duplicates reduces unnecessary growth.
