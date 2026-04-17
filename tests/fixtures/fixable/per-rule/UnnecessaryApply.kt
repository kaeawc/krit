package fixtures.positive.style

data class Config(var name: String = "", var version: String = "")

fun setup(): Config {
    val config = Config()
    config.apply { }
    return config
}

fun externalOnly(): Config {
    val config = Config()
    val x = "hello"
    config.apply { println(x) }
    return config
}
