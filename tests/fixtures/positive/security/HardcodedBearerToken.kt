package test

fun send(request: Request) {
    request.header("Authorization", "Bearer sk_live_abcdef0123456789")
}
