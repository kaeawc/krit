package fixable.style

class Builder {
    var name = ""
    var size = 0
    fun configure() {}
}

fun build(): Builder {
    return Builder().also {
        it.name = "x"
        it.size = 1
        it.configure()
    }
}
