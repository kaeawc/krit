package dev.krit.firsample.app

import dev.krit.firsample.model.Profile

// Findings that the Gradle and the Bazel runs must report the same way (#1677):
// one from source alone, and one that rests on Profile's types from :model,
// which only a run with the full classpath can confirm.
fun parityProbe(profile: Profile, counts: Map<String, Int>): Int {
    val unused = profile.id
    val length = profile.displayName?.length ?: 0
    return length + counts["visits"]!!
}
