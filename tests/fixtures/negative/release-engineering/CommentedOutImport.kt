package com.example.releaseengineering

import com.example.current.FeatureFlag

// Legacy import was removed during refactor.
// import order matters here
// import these constants before calling foo()
// import statements are sorted by ktlint

/*
 Old approach:
 // import com.example.legacy.Foo
*/

fun currentFlag(flag: FeatureFlag): FeatureFlag {
    val sample = """
        // import com.example.Foo
        class Foo
    """.trimIndent()
    println(sample)
    return flag
}
