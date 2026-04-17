# BitmapDecodeWithoutOptions

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`BitmapFactory.decodeFile/Resource/Stream(...)` without a
`BitmapFactory.Options` argument.

## Triggers

```kotlin
val bmp = BitmapFactory.decodeFile(path)
```

## Does not trigger

```kotlin
val options = BitmapFactory.Options().apply { inSampleSize = 2 }
val bmp = BitmapFactory.decodeFile(path, options)
```

## Dispatch

`call_expression` on `BitmapFactory.decode*` with a single argument.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
