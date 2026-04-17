# CommentedOutCodeBlock

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

≥ 3 consecutive `//` lines where each line parses as plausible
Kotlin (ends in `{`/`}`/`;` or contains `val `/`fun `/`if `/`when `).

## Triggers

```kotlin
// val result = compute()
// if (result > 0) {
//     render(result)
// }
```

## Does not trigger

Normal single-line `//` comments, ASCII art, or doc prose.

## Dispatch

Line rule with a heuristic parser.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
