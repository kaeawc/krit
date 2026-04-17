# CursorLoopWithColumnIndexInLoop

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`cursor.getColumnIndex(...)` inside a `while (cursor.moveToNext())`
loop — should hoist outside.

## Triggers

```kotlin
while (cursor.moveToNext()) {
    val name = cursor.getString(cursor.getColumnIndex("name"))
}
```

## Does not trigger

```kotlin
val nameIdx = cursor.getColumnIndex("name")
while (cursor.moveToNext()) {
    val name = cursor.getString(nameIdx)
}
```

## Dispatch

`while_statement` on `cursor.moveToNext` whose body contains
`getColumnIndex` calls.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
