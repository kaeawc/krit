package com.example

import dagger.Module
import dagger.Provides

@Module
object NetModule {
    @Provides
    fun provideBaseUrl(): String = "https://example.com"
}
