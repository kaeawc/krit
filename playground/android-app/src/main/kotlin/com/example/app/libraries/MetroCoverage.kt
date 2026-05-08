package com.example.app.libraries

import android.content.Context
import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.DependencyGraph
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.Qualifier
import dev.zacsweers.metro.Scope

@Scope
annotation class PlaygroundScope

@Qualifier
annotation class ApplicationContext

@DependencyGraph(PlaygroundScope::class)
interface PlaygroundGraph {
    val userRepository: PlaygroundUserRepository

    @DependencyGraph.Factory
    fun interface Factory {
        fun create(@ApplicationContext context: Context): PlaygroundGraph
    }
}

interface PlaygroundUserRepository {
    fun currentUser(): RemoteUser
}

@ContributesBinding(PlaygroundScope::class)
@Inject
class MetroUserRepository : PlaygroundUserRepository {
    override fun currentUser(): RemoteUser = RemoteUser(id = "1", name = "Metro User")
}

@ContributesTo(PlaygroundScope::class)
interface PlaygroundModule {
    @Provides
    fun provideNetworkCoverage(): NetworkLibraryCoverage = NetworkLibraryCoverage()
}
