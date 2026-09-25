// Smoke: a Metro dependency graph with a binding container and graph extension.
package stubs

import dev.zacsweers.metro.AppScope
import dev.zacsweers.metro.Binds
import dev.zacsweers.metro.BindingContainer
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.GraphExtension
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.SingleIn
import dev.zacsweers.metro.createGraph

interface MetroRepo

@Inject
@SingleIn(AppScope::class)
class MetroRepoImpl : MetroRepo

@BindingContainer
interface MetroBindings {
    @Binds
    val MetroRepoImpl.bind: MetroRepo
}

abstract class MetroChildScope

@GraphExtension(MetroChildScope::class)
interface MetroChildGraph {
    val repo: MetroRepo

    @GraphExtension.Factory
    interface Factory {
        fun create(): MetroChildGraph
    }
}

@DependencyGraph(AppScope::class, bindingContainers = [MetroBindings::class])
interface MetroAppGraph {
    val repo: MetroRepo

    val childFactory: MetroChildGraph.Factory

    @Provides
    fun provideName(): String = "metro"
}

fun metroGraph(): MetroAppGraph = createGraph<MetroAppGraph>()
