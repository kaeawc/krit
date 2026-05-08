package dihygiene

class Lazy<T>
class Api { fun initial() = Unit }
annotation class Inject

// Lazy used inside a function body — genuinely deferred; not flagged.
class Deferred @Inject constructor(private val api: Lazy<Api>) {
    fun load() = api.get().initial()
}

// Direct injection — not flagged.
class Direct @Inject constructor(private val api: Api) {
    private val loaded = api.initial()
}
