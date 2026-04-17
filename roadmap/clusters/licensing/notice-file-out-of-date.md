# NoticeFileOutOfDate

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Project has a `NOTICE` file and the set of declared dependencies
includes artifacts whose required attribution text is missing from
the `NOTICE`.

## Triggers

Dependency `com.example:attrib-required-lib` listed in a curated
"requires notice" registry; its notice text is not in `NOTICE`.

## Does not trigger

NOTICE covers every attribution-required dependency.

## Dispatch

`BuildGraph` + registry + NOTICE text scan.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
