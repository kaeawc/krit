package dev.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class AnalysisMemoTest {

    @Test
    fun importSourcePathMissPopulatesCache() {
        val memo = AnalysisMemo()
        memo.importSourcePathByFqn["com.example.Foo"] = "/src/Foo.kt"

        assertTrue(memo.importSourcePathByFqn.containsKey("com.example.Foo"))
        assertEquals("/src/Foo.kt", memo.importSourcePathByFqn["com.example.Foo"])
    }

    @Test
    fun importSourcePathCachesNullForMissingSymbols() {
        val memo = AnalysisMemo()
        memo.importSourcePathByFqn["com.example.Missing"] = null

        assertTrue(memo.importSourcePathByFqn.containsKey("com.example.Missing"))
        assertNull(memo.importSourcePathByFqn["com.example.Missing"])
    }

    @Test
    fun importSourcePathHitReturnsCachedValue() {
        val memo = AnalysisMemo()
        memo.importSourcePathByFqn["com.example.Bar"] = "/src/Bar.kt"

        // Second access should return same value (hit)
        val result = memo.importSourcePathByFqn["com.example.Bar"]
        assertEquals("/src/Bar.kt", result)
    }

    @Test
    fun renderedTypeCacheMissPopulatesOnFirstAccess() {
        val memo = AnalysisMemo()
        var computeCount = 0

        val result = memo.renderedTypeByKey.getOrPut("kotlin.String") {
            computeCount++
            "kotlin.String"
        }

        assertEquals("kotlin.String", result)
        assertEquals(1, computeCount)
    }

    @Test
    fun renderedTypeCacheHitSkipsRecomputation() {
        val memo = AnalysisMemo()
        var computeCount = 0

        // First access: miss
        memo.renderedTypeByKey.getOrPut("kotlin.String") {
            computeCount++
            "kotlin.String"
        }
        // Second access: hit
        memo.renderedTypeByKey.getOrPut("kotlin.String") {
            computeCount++
            "kotlin.String"
        }

        assertEquals(1, computeCount)
    }

    @Test
    fun separateKeysAreStoredIndependently() {
        val memo = AnalysisMemo()
        memo.importSourcePathByFqn["com.example.A"] = "/src/A.kt"
        memo.importSourcePathByFqn["com.example.B"] = "/src/B.kt"
        memo.importSourcePathByFqn["com.example.C"] = null

        assertEquals("/src/A.kt", memo.importSourcePathByFqn["com.example.A"])
        assertEquals("/src/B.kt", memo.importSourcePathByFqn["com.example.B"])
        assertNull(memo.importSourcePathByFqn["com.example.C"])
        assertEquals(3, memo.importSourcePathByFqn.size)
    }

    @Test
    fun annotationFqnsCacheStoresAndRetrieves() {
        val memo = AnalysisMemo()
        val annotations = listOf("kotlin.Deprecated", "java.lang.Override")
        memo.annotationFqnsByKey["com.example.Foo"] = annotations

        assertEquals(annotations, memo.annotationFqnsByKey["com.example.Foo"])
    }

    @Test
    fun freshMemoStartsEmpty() {
        val memo = AnalysisMemo()
        assertTrue(memo.importSourcePathByFqn.isEmpty())
        assertTrue(memo.renderedTypeByKey.isEmpty())
        assertTrue(memo.annotationFqnsByKey.isEmpty())
    }
}
