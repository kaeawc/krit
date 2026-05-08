package style

data class MutableUser(var name: String, var age: Int)

data class PartiallyMutable(val id: Int, var status: String)
