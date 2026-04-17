# SpdxIdentifierMismatchWithProject

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

File SPDX id differs from the project-level `license` setting in
`krit.yml`.

## Triggers

Project declares `Apache-2.0`; a file declares `MIT`.

## Does not trigger

All files agree with the project license.

## Dispatch

Header scan + project config lookup.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
