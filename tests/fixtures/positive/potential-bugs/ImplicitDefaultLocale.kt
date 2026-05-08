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
        String.format("%,d", 1000)
        String.format("%.2f", 1.0)
        String.format("%tF", java.util.Date())
    }

    fun formatExtension() {
        "%d".format(1)
        "%.2f".format(1.0)
        "Timestamp: %d".format(System.currentTimeMillis())
    }
}
