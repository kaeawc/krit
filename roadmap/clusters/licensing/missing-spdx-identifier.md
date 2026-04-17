# MissingSpdxIdentifier

**Cluster:** [licensing](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

File header comment that doesn't contain
`SPDX-License-Identifier: <id>`.

## Triggers

```kotlin
/*
 * Copyright 2024 Example
 */
package com.example
```

## Does not trigger

```kotlin
/*
 * Copyright 2024 Example
 * SPDX-License-Identifier: Apache-2.0
 */
package com.example
```

## Dispatch

File header scan; compares against the project `licensing:` block
in `krit.yml`.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
