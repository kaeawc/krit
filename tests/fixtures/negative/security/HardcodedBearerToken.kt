package test

object BuildConfig {
    const val API_TOKEN = "sk_live_abcdef0123456789"
}

fun send(request: Request) {
    request.header("Authorization", "Bearer ${BuildConfig.API_TOKEN}")
    request.header("Authorization", "Bearer your_api_token_here")
}
