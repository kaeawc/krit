# ScrollViewCount

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** shipped ·
**Severity:** warning · **Default:** active

## What it catches

`ScrollView` or `HorizontalScrollView` containers that have more than one direct child. The Android `ScrollView` contract requires exactly one direct child (typically a `LinearLayout` wrapping multiple items); multiple direct children cause a crash at runtime with `ScrollView can host only one direct child`. The equivalent Compose pattern is a `ScrollView`-like container wrapping a `Column` or `Row` with siblings added outside it.

Krit implements this through a source heuristic for scrollable Compose trees
and a resource rule for XML layouts.

## Example — triggers

```kotlin
// Compose heuristic — multiple sibling composables at the scroll root
@Composable
fun BadScrollContent() {
    Column(modifier = Modifier.verticalScroll(rememberScrollState())) {
        Text("Item 1")
        Text("Item 2")
        // This is fine in Compose; the XML case is the primary concern:
    }
}

// XML equivalent that the rule is designed to catch (handled by XML scanner):
// <ScrollView>
//     <TextView android:text="one" />
//     <TextView android:text="two" />   <!-- second direct child — crashes -->
// </ScrollView>
```

## Example — does not trigger

```kotlin
// Compose — single scrollable root is idiomatic and safe
@Composable
fun GoodScrollContent() {
    Column(modifier = Modifier.verticalScroll(rememberScrollState())) {
        repeat(50) { index ->
            Text("Item $index")
        }
    }
}
```

## Implementation notes

- Dispatch: `call_expression` (Compose) and XML element/resource data for the
  layout scanner
- Infra reuse: `internal/rules/android_correctness.go`,
  `internal/rules/android_resource_layout.go`, and `internal/android/`
- Coverage: source unit tests plus resource tests cover the concrete callbacks
- Related: `ScrollViewChildCountDetector` (AOSP), layout XML scanner

## Links

- Parent overview: [`../README.md`](../README.md)
