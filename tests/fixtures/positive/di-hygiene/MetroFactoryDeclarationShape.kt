package dihygiene

annotation class DependencyGraph {
    annotation class Factory
}

annotation class GraphExtension {
    annotation class Factory
}

interface AppGraph

@DependencyGraph.Factory
class AppGraphFactory {
    fun create(): AppGraph = TODO()
}

@DependencyGraph.Factory
object ObjectGraphFactory {
    fun create(): AppGraph = TODO()
}

@GraphExtension.Factory
sealed interface SealedExtensionFactory {
    fun create(): AppGraph
}
