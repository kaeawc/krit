package fixtures.negative.style

data class Config(var name: String = "", var version: String = "")

fun setup(): Config {
    val config = Config()
    config.apply { name = "foo" }
    return config
}

fun multipleMembers(): Config {
    val config = Config()
    config.apply {
        name = "foo"
        version = "1.0"
    }
    return config
}

fun explicitThis(): Config {
    val config = Config()
    config.apply { this.name = "bar" }
    return config
}

class Transition {
    fun setEvaluator(e: Any) {}
    fun setDuration(d: Long) {}
}

fun bareFunctionCallInApply(): Transition {
    return Transition().apply {
        setEvaluator(object {})
        setDuration(300)
    }
}

fun singleBareFunctionCallInApply(): Transition {
    return Transition().apply {
        setEvaluator(object {})
    }
}
