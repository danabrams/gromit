# Pre-existing Test Failures

## 1. TestRULESMD_ProcessSection

**File:** `cmd/gromit/bead_sizing_docs_test.go:73`
**Message:** `RULES.md should contain updated language about '6+ files across unrelated packages'`

**Root cause:** The documentation sync test expects `RULES.md` to contain specific language about file limits that hasn't been added yet. The test enforces that code changes to bead sizing rules are mirrored in `RULES.md`, but the corresponding doc update is missing.

## 2. TestAllDocuments_ConsistentFileLimits

**File:** `cmd/gromit/bead_sizing_docs_test.go:285`
**Message:** `.gromit/RULES.md: MISSING: no mention of new file limits`

**Root cause:** Same underlying issue. The test scans `.gromit/RULES.md` for mentions of file limit thresholds and finds none. A bead sizing rule was implemented in code without the corresponding `RULES.md` documentation update.

## Resolution

Both failures require adding the file limit language to `.gromit/RULES.md` to match the implemented bead sizing behavior. These are documentation sync issues, not code bugs.
