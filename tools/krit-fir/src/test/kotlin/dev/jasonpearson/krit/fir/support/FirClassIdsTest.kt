package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

class FirClassIdsTest {
    @Test
    fun stringPackageBuildsTopLevelClassId() {
        val id = classIdOf("java.util", "Map")
        assertEquals(ClassId(FqName("java.util"), Name.identifier("Map")), id)
        assertEquals("java/util/Map", id.asString())
        assertTrue(!id.isNestedClass)
    }

    @Test
    fun fqNamePackageMatchesStringOverload() {
        assertEquals(classIdOf("kotlin.collections", "List"), classIdOf(FqName("kotlin.collections"), "List"))
    }

    @Test
    fun emptyPackageIsRoot() {
        assertEquals("Foo", classIdOf("", "Foo").asString())
    }
}
