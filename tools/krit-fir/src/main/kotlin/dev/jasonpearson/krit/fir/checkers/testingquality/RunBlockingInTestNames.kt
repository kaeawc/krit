package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.support.GoTestAssertionNames

internal object RunBlockingInTestNames {
    fun assertionToken(name: String): Boolean =
        name != "expectError" && name !in GoTestAssertionNames.turbineAssertionCalls &&
            GoTestAssertionNames.equivalentTokens.any { containsToken(name, it) }

    private fun containsToken(name: String, token: String): Boolean {
        val lower = name.lowercase()
        var offset = lower.indexOf(token)
        while (offset >= 0) {
            val end = offset + token.length
            if (before(name, offset) && after(name, end)) return true
            offset = lower.indexOf(token, end)
        }
        return false
    }

    private fun before(name: String, offset: Int): Boolean =
        offset == 0 || !identifier(name[offset - 1]) ||
            name[offset - 1] in 'a'..'z' && name[offset] in 'A'..'Z'

    private fun after(name: String, offset: Int): Boolean =
        offset == name.length || !identifier(name[offset]) || name[offset] in 'A'..'Z'

    private fun identifier(character: Char): Boolean =
        character in 'a'..'z' || character in 'A'..'Z' || character in '0'..'9'
}
