package androidx.navigation

open class NavType<T> {
    companion object {
        val IntType: NavType<Int> = NavType()
    }
}

class NavArgumentBuilder {
    var type: NavType<*> = NavType.IntType
}

class NamedNavArgument

fun navArgument(name: String, builder: NavArgumentBuilder.() -> Unit): NamedNavArgument = TODO()
