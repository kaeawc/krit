package dihygiene

annotation class DependencyGraph {
    annotation class Factory
}

annotation class GraphExtension {
    annotation class Factory
}

interface AppGraph

@DependencyGraph.Factory
interface AppGraphFactory {
    fun create(): AppGraph
}

@GraphExtension.Factory
abstract class OtherGraphFactory {
    abstract fun create(): AppGraph
}

// No Metro factory annotation; not flagged.
class PlainFactory {
    fun create(): AppGraph = TODO()
}
