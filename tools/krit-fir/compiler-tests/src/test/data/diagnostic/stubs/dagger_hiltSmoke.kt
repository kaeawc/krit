// Smoke: a Hilt module installed in SingletonComponent plus an @EntryPoint.
package stubs

import dagger.Module
import dagger.Provides
import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object HiltNetworkModule {
    @Provides
    @Singleton
    fun provideBaseUrl(): String = "https://example.com"
}

@EntryPoint
@InstallIn(SingletonComponent::class)
interface HiltSmokeEntryPoint {
    fun baseUrl(): String
}
