package dev.jasonpearson.krit.types

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class CrossFileContextTest {

    private fun ctx(
        declarations: List<String> = emptyList(),
        nonCommentRefsByName: List<String> = emptyList(),
    ) = PayloadCrossFileContext(
        CrossFileProfilePayload(
            declarations = declarations,
            nonCommentRefsByName = nonCommentRefsByName,
        ),
    )

    @Test
    fun declarationLookupParsesPipeDelimitedEntries() {
        val crossFile = ctx(declarations = listOf("com.acme.Foo|class|/abs/Foo.kt|10|public"))
        val decl = assertNotNull(crossFile.declarationByFqn("com.acme.Foo"))
        assertEquals("com.acme.Foo", decl.fqn)
        assertEquals("class", decl.kind)
        assertEquals("/abs/Foo.kt", decl.file)
        assertEquals(10, decl.line)
        assertEquals("public", decl.visibility)
    }

    @Test
    fun missingDeclarationReturnsNull() {
        val crossFile = ctx(declarations = listOf("com.acme.Foo|class|/F.kt|1|"))
        assertNull(crossFile.declarationByFqn("com.acme.Missing"))
    }

    @Test
    fun declarationWithEmptyVisibilityNormalisesToNull() {
        val crossFile = ctx(declarations = listOf("com.acme.Foo|class|/abs/Foo.kt|10|"))
        val decl = assertNotNull(crossFile.declarationByFqn("com.acme.Foo"))
        assertNull(decl.visibility)
    }

    @Test
    fun malformedDeclarationEntriesAreIgnored() {
        val crossFile = ctx(
            declarations = listOf(
                "no-pipes",
                "two|pipes|only",
                "ok|class|/F.kt|not-an-int|",
                "com.acme.Good|class|/G.kt|5|",
            ),
        )
        assertNotNull(crossFile.declarationByFqn("com.acme.Good"))
        assertNull(crossFile.declarationByFqn("ok"))
    }

    @Test
    fun referenceLookupReturnsFileListInWireOrder() {
        val crossFile = ctx(
            nonCommentRefsByName = listOf("Foo|/abs/Bar.kt,/abs/Baz.kt"),
        )
        assertEquals(listOf("/abs/Bar.kt", "/abs/Baz.kt"), crossFile.referenceFiles("Foo"))
        assertTrue(crossFile.isReferenced("Foo"))
    }

    @Test
    fun unreferencedNameReturnsEmptyList() {
        val crossFile = ctx(nonCommentRefsByName = listOf("Foo|/abs/Bar.kt"))
        assertEquals(emptyList(), crossFile.referenceFiles("Missing"))
        assertFalse(crossFile.isReferenced("Missing"))
    }

    @Test
    fun parsesCrossFileProfileViaParseRequest() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt","crossFile":{"declarations":["c.D|class|/D.kt|1|"],"nonCommentRefsByName":["D|/U.kt"]}}}"""
        val req = parseRequest(json)
        val crossFile = req.crossFileProfile ?: error("crossFileProfile must round-trip")
        assertEquals(listOf("c.D|class|/D.kt|1|"), crossFile.declarations)
        assertEquals(listOf("D|/U.kt"), crossFile.nonCommentRefsByName)
    }

    @Test
    fun parseRequestLeavesCrossFileProfileNullWhenAbsent() {
        val json = """{"id":1,"method":"analyzeFile","params":{"path":"X.kt"}}"""
        assertNull(parseRequest(json).crossFileProfile)
    }
}
