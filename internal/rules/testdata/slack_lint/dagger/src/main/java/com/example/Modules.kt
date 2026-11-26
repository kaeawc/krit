package com.example

import dagger.Binds
import dagger.Module
import dagger.Provides
import javax.inject.Named

interface Foo
class FooImpl : Foo

@Provides fun topLevel(): Foo = FooImpl()

class NotAModule {
  @Provides fun provide(): Foo = FooImpl()

  @dagger.Provides fun fullyQualified(): Foo = FooImpl()

  fun outer() {
    // A local function is not a method, so it is never checked.
    @Provides fun local(): Foo = FooImpl()
  }

  val anonymous = object {
    @Provides fun inAnonymousObject(): Foo = FooImpl()
  }

  companion object {
    @Provides fun inCompanionOfNonModule(): Foo = FooImpl()
  }
}

@Module
interface InterfaceModule {
  @Binds fun bindWithBlockBody(impl: FooImpl): Foo { return impl }
  @Binds fun bindWithExpressionBody(impl: FooImpl): Foo = impl
  @Provides fun provideInInterface(): Foo
  @Binds fun bindOk(impl: FooImpl): Foo
  @Binds fun bindUnit(impl: FooImpl): Unit
  @Binds fun FooImpl.bindReceiver(): Foo
  // Wrong parameter counts are BindsWrongParameterCount (BindsMismatchedArity in krit), which hides later checks.
  @Binds fun bindNoParams(): Unit
  @Binds fun FooImpl.bindReceiverAndParam(other: FooImpl): Foo
  @Binds @Named("a") fun bindQualified(@Named("b") impl: Foo): Foo

  companion object {
    @Provides fun provideFromInterfaceCompanion(): Foo = FooImpl()
  }
}

@Module
abstract class AbstractModule {
  @Binds fun bindConcrete(impl: FooImpl): Foo = impl
  @Provides abstract fun provideAbstract(): Foo
  @Binds abstract fun bindOk(impl: FooImpl): Foo
  @Binds abstract fun bindAbstractUnit(impl: FooImpl)

  companion object {
    @Provides fun provideFromCompanion(): Foo = FooImpl()
    @Provides fun provideBlockUnit() {}
    @Provides fun provideExplicitUnit(): Unit {}
    @Provides fun provideQualifiedUnit(): kotlin.Unit {}
    @Provides fun String.provideReceiver(): Foo = FooImpl()
    @Provides fun String?.provideNullableReceiver(): Foo = FooImpl()
    @Provides fun <T> List<T>.provideGenericReceiver(): Foo = FooImpl()
  }

  object Nested {
    @Provides fun provideInNestedObject(): Foo = FooImpl()
  }
}

@Module
object ObjectModule {
  @Provides fun provideOk(): Foo = FooImpl()

  @Provides
  @JvmStatic
  fun provideMultiline(
    a: String,
  ): Foo = FooImpl()

  @Provides fun provideExpressionUnit() = Unit

  // Known false negative: the type of an expression body is not known from source.
  @Provides fun provideUnitCall() = println("x")

  @Provides fun provideInferred() = FooImpl()

  @Binds fun bindInObject(impl: FooImpl): Foo = impl
}

@Module
enum class EnumModule {
  A;

  @Binds fun bindInEnum(impl: FooImpl): Foo = impl
}

class Documented {
  /** krit anchors at the declaration; Lint starts the location at the KDoc. */
  @Provides fun withKdoc(): Foo = FooImpl()

  // Lint also binds a comment on the line before.
  @Provides
  fun withComment(): Foo = FooImpl()

  @Suppress("MustBeInModule")
  @Provides fun suppressed(): Foo = FooImpl()
}
