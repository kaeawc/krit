package dev.krit.firsample.model

/**
 * Marks API that callers must opt in to. This module opts in for all of its
 * sources through the compiler's `-opt-in` option instead of `@OptIn`, so FIR
 * compiles [Profile] without errors only when it applies the module's
 * compiler settings.
 */
@RequiresOptIn(level = RequiresOptIn.Level.ERROR)
@Retention(AnnotationRetention.BINARY)
@Target(AnnotationTarget.CLASS, AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY)
annotation class ExperimentalSampleApi
