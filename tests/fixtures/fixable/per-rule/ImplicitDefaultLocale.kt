package potentialbugs

import java.util.Locale

class ImplicitDefaultLocale {
    fun shout(s: String): String {
        return s.toUpperCase()
    }

    fun whisper(s: String): String {
        return s.toLowerCase()
    }
}
