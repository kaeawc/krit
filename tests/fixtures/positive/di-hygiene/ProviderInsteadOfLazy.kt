package dihygiene

class Provider<T>
class Api { fun fetch() = Unit }
annotation class Inject

class Presenter @Inject constructor(
    private val api: Provider<Api>,
) {
    fun load() = api.get().fetch()
}
