package test

class UserLookup {
    fun load(resolver: Any, uri: Any, name: String) {
        resolver.query(uri, null, "name = ?", arrayOf(name), null)
    }
}
