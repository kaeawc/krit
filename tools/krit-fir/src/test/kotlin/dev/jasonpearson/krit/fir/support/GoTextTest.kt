package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class GoTextTest {
    @Test
    fun trimsGoSpaceButNotKotlinOnlySeparators() {
        assertEquals("a", trimGoSpace("\u0085  a　 \n"))
        assertEquals("\u001Ca\u001F", trimGoSpace(" \u001Ca\u001F "))
    }

    @Test
    fun keepsInteriorSpace() {
        assertEquals("a b", trimGoSpace(" a b "))
    }

    @Test
    fun upperCasesBySimpleMapping() {
        assertEquals("SHA-256", upperCodePoints("sha-256"))
        assertEquals("ß", upperCodePoints("ß"))
        assertEquals("𐐀", upperCodePoints("𐐨"))
    }

    @Test
    fun fullMatchRegexAnchorsAPlainPattern() {
        val regex = assertNotNull(goFullMatchRegex("ignored"))
        assertTrue(regex.matches("ignored"))
        assertFalse(regex.matches("ignoredToo"))
        assertFalse(regex.matches("notignored"))
    }

    @Test
    fun fullMatchRegexGroupsATopLevelAlternation() {
        val regex = assertNotNull(goFullMatchRegex("ignored|expected"))
        assertTrue(regex.matches("expected"))
        assertFalse(regex.matches("expected2"))
        assertFalse(regex.matches("2ignored"))
    }

    @Test
    fun fullMatchRegexKeepsExistingAnchors() {
        assertEquals("^a|b$", goFullMatchRegex("^a|b$")?.pattern)
        assertEquals("^(?:a|b)$", goFullMatchRegex("^a|b")?.pattern)
    }

    @Test
    fun fullMatchRegexRejectsAnInvalidPattern() {
        assertNull(goFullMatchRegex("("))
    }
}
