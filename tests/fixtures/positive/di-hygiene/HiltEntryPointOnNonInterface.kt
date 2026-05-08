import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent

@EntryPoint
@InstallIn(SingletonComponent::class)
class LegacyEntryPoint {
    fun api(): Api = TODO()
}

interface Api
