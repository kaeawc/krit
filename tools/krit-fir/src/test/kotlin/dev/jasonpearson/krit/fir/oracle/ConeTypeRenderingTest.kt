package dev.jasonpearson.krit.fir.oracle

import org.jetbrains.kotlin.fir.types.ConeStarProjection
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.name.StandardClassIds
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

class ConeTypeRenderingTest {

    @Test
    fun starProjectionRendersAsStarNotAnObjectIdentity() {
        val list = StandardClassIds.List.constructClassLikeType(arrayOf(ConeStarProjection), isMarkedNullable = true)
        assertEquals("kotlin.collections.List<*>?", list.renderFqn())
    }

    @Test
    fun nestedStarProjectionsRenderStably() {
        val map = StandardClassIds.Map.constructClassLikeType(arrayOf(ConeStarProjection, ConeStarProjection))
        assertEquals("kotlin.collections.Map<*, *>", map.renderFqn())
    }
}
