package com.example

import dagger.Binds
import dagger.Module
import dagger.Provides

interface Foo
class FooImpl : Foo

@Module
object FooModule {
  @Provides fun provideFoo(): Foo = FooImpl()
  fun helper(): Int = 1
}

@Module
abstract class BindsModule {
  @Binds abstract fun bindFoo(impl: FooImpl): Foo

  companion object {
    @Provides fun provideName(): String = "x"
  }
}

class User {
  fun use() {
    FooModule.provideFoo()
    FooModule.helper()
    BindsModule.provideName()
    val m = FooModule
    m.provideFoo()
  }
}

@javax.annotation.Generated("dagger")
class GeneratedUser {
  fun use() { FooModule.provideFoo() }
}

class Lookalike {
  @com.example.other.Provides fun provideFoo(): Foo = FooImpl()
}

class OtherUser(private val lookalike: Lookalike) {
  fun use() {
    lookalike.provideFoo()
    val ref = FooModule::provideFoo
    with(FooModule) { provideFoo() }
  }
}

@Module
class InstanceModule {
  @Provides fun provideCount(): Int = 1
  @Provides fun provideTwice(): Int = provideCount() * 2
}
