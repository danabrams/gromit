# Vision Metrics

spec_id: spec-2026-000
cycle_start_trigger_at: 2026-02-25T08:00:00Z
cycle_end_presented_at: 2026-02-28T16:00:00Z
review_outcome: accepted | rework_implementation_gap | rework_vision_change
review_rationale: (required when review_outcome is rework_vision_change; replace with short explanation when the scope of the work changes vision)
human_tactical_intervention: yes | no
human_debugging_intervention: yes | no
escaped_regression_within_7d: yes | no | pending

review_rationale is required when review_outcome is rework_vision_change; add a few words summarizing how the product direction shifted so downstream rollups stay auditable.

### Known-good Vision Metrics example (LLM copy-edit safe)

# Vision Metrics

spec_id: spec-2026-011
cycle_start_trigger_at: 2026-02-24T10:00:00Z
cycle_end_presented_at: 2026-02-27T14:00:00Z
review_outcome: rework_vision_change
review_rationale: Product owner shifted direction after the first presentation.
human_tactical_intervention: yes
human_debugging_intervention: no
escaped_regression_within_7d: pending
