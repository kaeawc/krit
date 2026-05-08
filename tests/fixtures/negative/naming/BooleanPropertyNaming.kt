package naming

class Example {
    val isEnabled: Boolean = true
    val hasPermission: Boolean = false
    val areValid: Boolean = true

    // Non-Boolean declared types initialized with true/false must not be flagged.
    val flag: Any = true
    val marker: Any = false
    val token: Comparable<*> = true
}
