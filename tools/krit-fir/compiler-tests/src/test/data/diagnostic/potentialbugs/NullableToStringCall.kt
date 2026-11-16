// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 17, 21, 25, 30, 34, 38, 42, 46x2, 50
package test

class Repo {
    fun findName(useFallback: Boolean): String? = if (useFallback) "fallback" else null
}

typealias MaybeName = String?

class Positives {
    fun directCall(value: String?): String {
        return <!NullableToStringCall!>value.toString()<!>
    }

    fun callResult(repo: Repo): String {
        return <!NullableToStringCall!>repo.findName(false).toString()<!>
    }

    fun parenthesized(value: String?): String {
        return <!NullableToStringCall!>(value).toString()<!>
    }

    fun anyReceiver(value: Any?): String {
        return <!NullableToStringCall!>value.toString()<!>
    }

    fun inferredNullable(flag: Boolean): String {
        val value = if (flag) "ok" else null
        return <!NullableToStringCall!>value.toString()<!>
    }

    fun templateIdentifier(value: String?): String {
        return "value=<!NullableToStringCall!>$value<!>"
    }

    fun templateExpression(repo: Repo): String {
        return "name=<!NullableToStringCall!>${repo.findName(true)}<!>"
    }

    fun templateSafeCall(value: String?): String {
        return "length=<!NullableToStringCall!>${value?.length}<!>"
    }

    fun twoParts(first: String?, second: String?): String {
        return "<!NullableToStringCall!>$first<!>-<!NullableToStringCall!>$second<!>"
    }

    fun aliasedNullable(value: MaybeName): String {
        return <!NullableToStringCall!>value.toString()<!>
    }
}
