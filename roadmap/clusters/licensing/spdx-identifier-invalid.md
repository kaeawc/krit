# SpdxIdentifierInvalid

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

`SPDX-License-Identifier:` value that isn't a recognised SPDX
short ID.

## Triggers

```kotlin
/* SPDX-License-Identifier: Apache2 */
```

## Does not trigger

```kotlin
/* SPDX-License-Identifier: Apache-2.0 */
```

## Dispatch

Header scan against the embedded SPDX ID list.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
