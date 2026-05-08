package test

class RemoteConfig {
    fun setDefaultsAsync(defaults: Map<String, Any>) {}
}

fun setup(config: RemoteConfig) {
    config.setDefaultsAsync(mapOf(
        "user_email_template" to "%s@example.com",
    ))
}
