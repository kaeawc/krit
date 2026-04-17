package fixtures.positive.potentialbugs

class ImplicitDefaultLocale {
    fun convert(str: String): String {
        return str.toLowerCase()
    }

    fun upper(str: String): String {
        return str.toUpperCase()
    }

    fun formatString() {
        String.format("%d", 1)
    }

    fun formatExtension() {
        "%d".format(1)
    }
}
