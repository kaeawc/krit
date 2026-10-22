package dev.jasonpearson.krit.intellij

/**
 * Pure helpers for the "Scan debounce (ms)" setting. Kept free of IntelliJ
 * classes so the clamp and parse rules are unit-testable without the platform.
 */
object KritScanDebounce {
    const val DEFAULT_MS = 1_000
    const val MIN_MS = 100

    /** Applies the floor so a bad persisted or typed value cannot cause runaway scans. */
    fun clamp(ms: Int): Int = ms.coerceAtLeast(MIN_MS)

    /** The delay to schedule with: [configured] clamped, or the default when unset. */
    fun effectiveMs(configured: Int?): Long = clamp(configured ?: DEFAULT_MS).toLong()

    /** Parses form input; null when blank, non-numeric, or out of Int range. */
    fun parse(text: String): Int? = text.trim().toIntOrNull()

    /** Form input to a persistable value: clamped when numeric, default otherwise. */
    fun fromInput(text: String): Int = parse(text)?.let(::clamp) ?: DEFAULT_MS
}
