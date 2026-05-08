package test

class RemoteConfig {
    fun setDefaultsAsync(defaults: Map<String, Any>) {}
}

fun setup(config: RemoteConfig) {
    config.setDefaultsAsync(mapOf(
        "welcome_message" to "Hello",
    ))
}
