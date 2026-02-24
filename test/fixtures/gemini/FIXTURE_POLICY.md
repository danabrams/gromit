# Gemini Fixture Policy (Temporary)

This fixture set follows the same deterministic style as Codex fixtures in `test/fixtures/`:

- Commit curated fixture transcripts, not full raw dumps.
- Keep fixtures stable and contract-shaped for tests.
- Add `# provenance:` and `# refresh:` headers to markdown/log/jsonl artifacts where comments are allowed.
- Sanitize fixture content by construction:
  - no absolute host-specific paths when not required for behavior evidence
  - no usernames or environment-derived secrets
  - no credentials, tokens, or auth material

This policy is temporary and test-focused. Once provider behavior is validated and stable, fixture retention can be reduced to the minimum needed for contract coverage.
