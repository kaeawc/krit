package dev.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Pure-unit tests for the resolveExpressionTypes daemon RPC. Covers
 * request parsing and response building — the parts that don't need a
 * KaSession. End-to-end resolution against real Kotlin code is
 * exercised when PR C wires the RPC into scan; here we pin the wire
 * format so the Go side and JVM side stay in sync.
 */
class ResolveExpressionTypesTest {

    @Test
    fun parserExtractsSinglePosition() {
        val json = """{"expressionPositions": {"/a.kt": [{"line": 7, "col": 13}]}}"""
        val map = extractJsonExpressionPositionsMap(json, "expressionPositions")
        assertNotNull(map)
        assertEquals(listOf(RequestExpressionPosition(7, 13)), map["/a.kt"])
    }

    @Test
    fun parserExtractsMultiplePositionsAcrossFiles() {
        val json = """
            {"expressionPositions": {
                "/a.kt": [{"line": 1, "col": 2}, {"line": 5, "col": 9}],
                "/b.kt": [{"line": 3, "col": 4}]
            }}
        """.trimIndent()
        val map = extractJsonExpressionPositionsMap(json, "expressionPositions")
        assertNotNull(map)
        assertEquals(2, map.size)
        assertEquals(
            listOf(RequestExpressionPosition(1, 2), RequestExpressionPosition(5, 9)),
            map["/a.kt"]
        )
        assertEquals(listOf(RequestExpressionPosition(3, 4)), map["/b.kt"])
    }

    @Test
    fun parserReturnsNullWhenKeyAbsent() {
        val json = """{"id": 1, "method": "ping"}"""
        assertNull(extractJsonExpressionPositionsMap(json, "expressionPositions"))
    }

    @Test
    fun parserReturnsEmptyMapForEmptyObject() {
        val json = """{"expressionPositions": {}}"""
        val map = extractJsonExpressionPositionsMap(json, "expressionPositions")
        assertNotNull(map)
        assertTrue(map.isEmpty())
    }

    @Test
    fun parserHandlesEmptyArray() {
        val json = """{"expressionPositions": {"/a.kt": []}}"""
        val map = extractJsonExpressionPositionsMap(json, "expressionPositions")
        assertNotNull(map)
        assertEquals(emptyList(), map["/a.kt"])
    }

    @Test
    fun parserReturnsNullOnMissingLineField() {
        val json = """{"expressionPositions": {"/a.kt": [{"col": 5}]}}"""
        assertNull(extractJsonExpressionPositionsMap(json, "expressionPositions"))
    }

    @Test
    fun parserReturnsNullOnMissingColField() {
        val json = """{"expressionPositions": {"/a.kt": [{"line": 5}]}}"""
        assertNull(extractJsonExpressionPositionsMap(json, "expressionPositions"))
    }

    @Test
    fun parseRequestPopulatesExpressionPositionsField() {
        val json = """
            {"id": 42, "method": "resolveExpressionTypes",
             "expressionPositions": {"/a.kt": [{"line": 7, "col": 13}]}}
        """.trimIndent()
        val request = parseRequest(json)
        assertEquals("resolveExpressionTypes", request.method)
        assertEquals(42L, request.id)
        assertNotNull(request.expressionPositions)
        assertEquals(listOf(RequestExpressionPosition(7, 13)), request.expressionPositions["/a.kt"])
    }

    @Test
    fun parseRequestLeavesExpressionPositionsNullForOtherMethods() {
        val json = """{"id": 1, "method": "ping"}"""
        val request = parseRequest(json)
        assertNull(request.expressionPositions)
    }

    @Test
    fun responseBuilderEmptyResultsAndErrors() {
        val json = buildResolveExpressionTypesResponse(7L, emptyMap(), emptyMap())
        // Errors omitted entirely when empty.
        assertEquals("""{"id":7,"result":{"types":{}}}""", json)
    }

    @Test
    fun responseBuilderSingleFactSerializesCorrectly() {
        val byFile = mapOf(
            "/a.kt" to mapOf(
                "7:13" to ResolvedExpressionFact(name = "String", fqn = "kotlin.String", nullable = false)
            )
        )
        val json = buildResolveExpressionTypesResponse(1L, byFile, emptyMap())
        assertTrue(json.contains(""""types":{"/a.kt":{"7:13":{"name":"String","fqn":"kotlin.String","nullable":false}}}"""),
            "unexpected response shape: $json")
    }

    @Test
    fun responseBuilderEmitsNullableTrueWhenSet() {
        val byFile = mapOf(
            "/a.kt" to mapOf(
                "1:1" to ResolvedExpressionFact(name = "Int", fqn = "kotlin.Int", nullable = true)
            )
        )
        val json = buildResolveExpressionTypesResponse(2L, byFile, emptyMap())
        assertTrue(json.contains(""""nullable":true"""), "expected nullable:true in $json")
    }

    @Test
    fun responseBuilderIncludesErrorsWhenPresent() {
        val errors = mapOf("/missing.kt" to "File not found in source module")
        val json = buildResolveExpressionTypesResponse(3L, emptyMap(), errors)
        assertTrue(json.contains(""""errors":{"/missing.kt":"File not found in source module"}"""),
            "expected errors block in $json")
    }

    @Test
    fun responseBuilderEscapesQuotesInErrorMessages() {
        val errors = mapOf("/a.kt" to "Unexpected \"quote\" in payload")
        val json = buildResolveExpressionTypesResponse(4L, emptyMap(), errors)
        // escJsonStr should have escaped both quotes.
        assertTrue(json.contains("""Unexpected \"quote\" in payload"""),
            "expected escaped quotes in $json")
    }

    @Test
    fun responseBuilderRoundTripsMultipleFilesAndPositions() {
        val byFile = linkedMapOf(
            "/a.kt" to linkedMapOf(
                "1:1" to ResolvedExpressionFact("String", "kotlin.String", false),
                "5:7" to ResolvedExpressionFact("Int", "kotlin.Int", true)
            ),
            "/b.kt" to linkedMapOf(
                "3:2" to ResolvedExpressionFact("List", "kotlin.collections.List", false)
            )
        )
        val json = buildResolveExpressionTypesResponse(99L, byFile, emptyMap())
        assertTrue(json.contains("\"/a.kt\""))
        assertTrue(json.contains("\"/b.kt\""))
        assertTrue(json.contains("\"1:1\""))
        assertTrue(json.contains("\"5:7\""))
        assertTrue(json.contains("\"3:2\""))
        assertTrue(json.startsWith("""{"id":99,"""))
    }
}
