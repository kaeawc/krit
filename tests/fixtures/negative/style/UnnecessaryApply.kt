package fixtures.negative.style

class Settings {
    var name: String = ""
    var retries: Int = 0

    fun reset() {}
}

fun topLevel() {}

fun multipleStatements(settings: Settings) {
    settings.apply {
        this.name = "primary"
        this.retries = 3
    }
}

fun valueIsUsed(settings: Settings): Settings {
    val copy = settings.apply { this.reset() }
    println(copy)
    return settings.apply { this.reset() }
}

fun valueIsChained(settings: Settings) {
    settings.apply { this.reset() }.reset()
}

fun expressionBody(settings: Settings) = settings.apply { this.reset() }

fun resultOfLambda(settings: Settings) = run { settings.apply { this.reset() } }

fun noReceiverReference(settings: Settings) {
    settings.apply { topLevel() }
    settings.apply { println("done") }
}

fun otherReceiver(settings: Settings, other: Settings) {
    settings.apply { other.reset() }
}

fun emptyBlock(settings: Settings) {
    settings.apply { }
}

fun notAReferenceOrAssignment(settings: Settings) {
    settings.apply { this.retries + 1 }
    settings.apply { this?.reset() }
}

fun twoReceiverReferences(settings: Settings) {
    settings.apply { this.reset(); this.reset() }
}

fun implicitReceiverInScope(settings: Settings) {
    with(settings) {
        apply { this.reset() }
    }
}
