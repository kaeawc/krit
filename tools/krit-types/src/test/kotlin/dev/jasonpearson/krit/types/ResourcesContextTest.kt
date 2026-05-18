package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ResourcesContextTest {

    private fun ctx(
        strings: List<String> = emptyList(),
        drawables: List<String> = emptyList(),
        layouts: List<String> = emptyList(),
        colors: List<String> = emptyList(),
        dimensions: List<String> = emptyList(),
        ids: List<String> = emptyList(),
    ) = PayloadResourcesContext(
        ResourcesProfilePayload(
            strings = strings,
            drawables = drawables,
            layouts = layouts,
            colors = colors,
            dimensions = dimensions,
            ids = ids,
        ),
    )

    @Test
    fun stringValueLookupHonorsNameEqualsValueEncoding() {
        val resources = ctx(strings = listOf("app_name=Acme", "ok=OK"))
        assertEquals("Acme", resources.stringValue("app_name"))
        assertEquals("OK", resources.stringValue("ok"))
        assertNull(resources.stringValue("missing"))
        assertTrue(resources.hasString("app_name"))
        assertFalse(resources.hasString("missing"))
    }

    @Test
    fun colorAndDimensionValuesParseSimilarly() {
        val resources = ctx(
            colors = listOf("primary=#FF0000"),
            dimensions = listOf("margin_default=16dp"),
        )
        assertEquals("#FF0000", resources.colorValue("primary"))
        assertEquals("16dp", resources.dimensionValue("margin_default"))
        assertTrue(resources.hasColor("primary"))
        assertTrue(resources.hasDimension("margin_default"))
        assertFalse(resources.hasColor("missing"))
    }

    @Test
    fun stringValueWithEqualsSignPreservesEverythingAfterFirstEquals() {
        // Defensive — i18n strings may include `=` characters; the
        // parser splits on the first `=` only so values containing `=`
        // round-trip cleanly.
        val resources = ctx(strings = listOf("encoded_url=a=b&c=d"))
        assertEquals("a=b&c=d", resources.stringValue("encoded_url"))
    }

    @Test
    fun drawableLayoutIdLookupsAreSetBased() {
        val resources = ctx(
            drawables = listOf("ic_launcher"),
            layouts = listOf("activity_main", "fragment_home"),
            ids = listOf("root_view"),
        )
        assertTrue(resources.hasDrawable("ic_launcher"))
        assertFalse(resources.hasDrawable("missing"))
        assertTrue(resources.hasLayout("activity_main"))
        assertTrue(resources.hasLayout("fragment_home"))
        assertTrue(resources.hasId("root_view"))
        assertFalse(resources.hasId("missing"))
    }

    @Test
    fun malformedStringEntriesAreIgnored() {
        val resources = ctx(strings = listOf("no-equals", "=empty-name", "ok=OK"))
        assertEquals("OK", resources.stringValue("ok"))
        assertFalse(resources.hasString("no-equals"))
        assertFalse(resources.hasString(""))
    }

    @Test
    fun parsesResourcesProfileViaParseRequest() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt","resources":{"strings":["app_name=Acme"],"drawables":["ic_launcher"],"layouts":["activity_main"],"colors":["primary=#FF0000"],"dimensions":["margin=16dp"],"ids":["root"]}}}"""
        val req = parseRequest(json)
        val resources = req.resourcesProfile ?: error("resourcesProfile must round-trip")
        assertEquals(listOf("app_name=Acme"), resources.strings)
        assertEquals(listOf("ic_launcher"), resources.drawables)
        assertEquals(listOf("activity_main"), resources.layouts)
    }

    @Test
    fun parseRequestLeavesResourcesProfileNullWhenAbsent() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt"}}"""
        assertNull(parseRequest(json).resourcesProfile)
    }
}
