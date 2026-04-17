# Non-Android Fixture Completeness

**Cluster:** [rule-quality](./README.md) · **Status:** in-progress ·
**Supersedes:** [`roadmap/25-fixture-completeness.md`](../../25-fixture-completeness.md)

## What it is

Ensures every non-Android rule has both positive and negative `.kt` fixture
files in `tests/fixtures/`. Android-lint fixture gaps are tracked separately
in `android-lint/fixture-gaps.md`.

## Current state

Original gaps all closed as of 2026-04-14. 1,044 fixture files exist
(464 positive, 466 negative, 114 fixable). The remaining gap is primarily
Android-lint rules (tracked in the android-lint cluster).

## Implementation notes

- Fixture files: `tests/fixtures/positive/<category>/`, `tests/fixtures/negative/<category>/`
- Test runner: `go test ./internal/rules/ -run TestPositiveFixtures -v`
- Any new rule added must include fixtures (enforced by convention, not CI yet)
