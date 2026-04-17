# CopyrightYearOutdated

**Cluster:** [licensing](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`Copyright (c) 2018 ...` in a file whose last content-bearing
modification is after 2023 (git input).

## Triggers

Old copyright year on a file with recent git history.

## Does not trigger

Copyright year within the recent window, or file untouched.

## Dispatch

Header scan + optional git-log integration; CI-only.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
