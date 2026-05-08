package test

object Log {
    fun d(tag: String, msg: String) {}
}

fun debug() {
    Log.d("Auth", "token loaded")
}
