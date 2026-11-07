package fixtures.positive.style

class Settings {
    var name: String = ""
    var retries: Int = 0
    val tags = mutableListOf<String>()

    fun reset() {}
}

fun configure(settings: Settings, retries: Int) {
    settings.apply { this.name = "primary" }
    settings.apply { this.reset() }
    settings.apply { this.retries = retries }
    settings.apply { this.tags.add("fast") }
}

fun configureOptional(settings: Settings?) {
    settings?.apply { this.reset() }
}
