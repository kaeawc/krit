package style

fun example(nullable: String?): Int? {
    return nullable?.let { it.length }
}

fun withoutNullBranch(nullable: String?) {
    if (nullable != null) {
        nullable.length
    }
}
