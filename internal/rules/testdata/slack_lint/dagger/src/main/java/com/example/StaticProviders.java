package com.example;

import dagger.Binds;
import dagger.Module;
import dagger.Provides;
import dagger.producers.ProducerModule;

@Module
interface StaticProviders {
  @Provides
  static Foo provideStatic() { return null; }

  @Provides
  default Foo provideDefault() { return null; }
}

@ProducerModule
abstract class JavaProducerModule {
  @Binds abstract Foo bind(FooImpl impl);
  @Provides static Foo provide() { return null; }
}
