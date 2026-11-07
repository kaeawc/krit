package style

fun example(nullable: String?): Int? {
    return if (nullable != null) nullable.length else null
}
