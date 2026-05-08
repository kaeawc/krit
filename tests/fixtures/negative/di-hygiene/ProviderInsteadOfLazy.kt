package dihygiene

class Provider<T>
class Lazy<T>
class Api { fun fetch() = Unit }
annotation class Inject

// Provider used twice — keep Provider semantics (multiple new instances).
class TwoCalls @Inject constructor(
    private val api: Provider<Api>,
) {
    fun load() = api.get().fetch()
    fun reload() = api.get().fetch()
}

// Lazy already; not flagged.
class WithLazy @Inject constructor(
    private val api: Lazy<Api>,
) {
    fun load() = api.get().fetch()
}

// Direct injection; not flagged.
class Direct @Inject constructor(private val api: Api) {
    fun load() = api.fetch()
}
