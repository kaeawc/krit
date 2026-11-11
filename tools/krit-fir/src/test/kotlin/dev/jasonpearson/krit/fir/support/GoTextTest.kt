package dev.jasonpearson.krit.fir.support

import kotlin.test.Test
import kotlin.test.assertEquals

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
}
