package potentialbugs

class UnsafeCallOnNullableType {
    fun example(nullable: String?) {
        val len = nullable!!.length
    }

    fun bareAssert(nullable: String?) {
        val x = nullable!!
    }
}

class PostFilterUnsafePatterns {
    data class Item(val name: String?, val age: Int?, val firstName: String?, val first: String?)

    // Checks a different field than the one accessed with !!
    fun unsafeDifferentField(list: List<Item>): List<String?> {
        return list.filter { it.age != null }.map { it.name!! }
    }

    // Named lambda param in filter but accessing a different field outside
    fun unsafeNamedParamDifferentField(list: List<Item>): List<Int?> {
        return list.filter { item -> item.name != null }.map { it.age!! }
    }

    // Substring collision: filter checks "firstName" but !! accesses "first"
    fun unsafeSubstringCollision(list: List<Item>): List<String?> {
        return list.filter { it.firstName != null }.map { it.first!! }
    }
}

class SignalInspiredUnsafePatterns {
    class Controller {
        fun start() {}
    }

    private var controller: Controller? = null

    fun nextName(): String? = null

    fun repeatedFunctionCall(): Boolean {
        return nextName() != null && nextName()!!.isNotEmpty()
    }

    fun createController(): Controller? = null

    fun nullableFactoryAssignment() {
        controller = createController()
        controller!!.start()
    }

    fun overwrittenWithNull() {
        controller = Controller()
        controller = null
        controller!!.start()
    }
}
