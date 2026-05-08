package dihygiene

class Lazy<T>
class Api { fun initial() = Unit }
annotation class Inject

class Presenter @Inject constructor(private val api: Lazy<Api>) {
    private val loaded = api.get().initial()
}
