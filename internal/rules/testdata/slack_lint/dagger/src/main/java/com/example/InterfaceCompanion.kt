package com.example

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.producers.ProducerModule

@Module
interface InterfaceCompanionModule {
  companion object {
    @Provides @JvmStatic fun provideStatic(): Foo = FooImpl()
  }
}

@ProducerModule
interface KotlinProducerModule {
  @Binds fun bind(impl: FooImpl): Foo
}
