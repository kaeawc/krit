package fixtures.positive.style

fun redundant(): String {
    return "hello".orEmpty()
}

// NOTE: navigation_expression cases like c.value.orEmpty() where c.value is String
// are not covered here because the resolver does not track class member property types
// for user-defined classes — it cannot infer that Container.value is String without
// a full type index. These cases would require the oracle/Analysis API to resolve.
