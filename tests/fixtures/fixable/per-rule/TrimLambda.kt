package fixtures

fun clean(s: String, maybe: String?): List<String?> =
    listOf(
        s.trim { it <= ' ' },
        s.trim() { it <= ' ' },
        s.trim({ it <= ' ' }),
        maybe?.trim { it.isWhitespace() },
        s.trim { c -> c.isWhitespace() }.lowercase(),
    )
