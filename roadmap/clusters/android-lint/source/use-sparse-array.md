# UseSparseArray

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`HashMap<Integer, V>` (or `HashMap<Int, V>` in Kotlin) declarations where the key is a boxed integer type. On Android, `SparseArray<V>` avoids boxing overhead and reduces memory allocations, which matters on memory-constrained devices. Also flags `HashMap<Integer, Boolean>` → `SparseBooleanArray` and `HashMap<Integer, Integer>` → `SparseIntArray`.

## Example — triggers

```kotlin
class ContactsCache {
    // Integer key causes unnecessary boxing on every get/put
    private val cache = HashMap<Int, Contact>()

    fun put(id: Int, contact: Contact) { cache[id] = contact }
    fun get(id: Int): Contact? = cache[id]
}
```

## Example — does not trigger

```kotlin
class ContactsCache {
    private val cache = SparseArray<Contact>()

    fun put(id: Int, contact: Contact) { cache.put(id, contact) }
    fun get(id: Int): Contact? = cache[id]
}
```

## Implementation notes

- Dispatch: `call_expression` or `type_reference`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small
- Related: `JavaPerformanceDetector` (AOSP), `UseSparseArray`, `SparseIntArray`, `SparseBooleanArray`, `SparseLongArray`

## Links

- Parent overview: [`../README.md`](../README.md)
