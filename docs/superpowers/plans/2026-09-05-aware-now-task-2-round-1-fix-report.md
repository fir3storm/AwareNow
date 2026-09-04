# Task 2 Round 1 Fix Report

Date: 2026-09-05

## Scope

This round applies the reviewer findings for the Task 2 import only. It does
not change deployment identities or paths, frontend code, CI, or application
files.

## Files changed

- `scripts/keep-english-templates.py` — replaced with the cleanup-safe
  hardening implementation from worktree commit `a186ce4`.
- `scripts/tests/test_keep_english_templates.py` — added the focused
  regression tests.
- `docs/superpowers/specs/2026-09-04-awarenow-hardening-design.md` — imported
  hardening design.
- `docs/superpowers/plans/2026-09-04-awarenow-hardening.md` — imported
  hardening plan.
- `docs/superpowers/plans/2026-09-05-aware-now-task-2-round-1-fix-report.md`
  — this report.

The cleanup script now validates `GOPHISH_API_KEY` before local mutation when
`--gophish` is requested, retains manually created non-English records, and
deletes only names derived from retired HailBytes packs.

## Verification

Command:

```text
python -m unittest scripts.tests.test_keep_english_templates -v
```

Output:

```text
Ran 2 tests in 0.612s

OK
```

The test cases cover API-key validation before local deletion and retaining a
manually created Spanish-language record.

Command:

```text
git diff --check
```

Result: pass.

## Exclusions

No push was performed. Existing consolidation documents were preserved. The
test run generated only ignored Python cache files under `scripts/tests/`.
