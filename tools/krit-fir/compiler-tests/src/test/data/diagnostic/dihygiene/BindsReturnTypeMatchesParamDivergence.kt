// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 28, 33, 39, 45, 50, 56, 62, 68, 75, 81, 87
// Go findings FIR drops. Go matches `@Binds` as a substring of the modifier
// text and compares the parameter and return type text alone. None of these
// functions is a no-op binding.
package test

import dagger.Binds
import dagger.BindsInstance
import dagger.BindsOptionalOf
import dagger.Module
import dagger.multibindings.ElementsIntoSet
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoSet
import dagger.multibindings.StringKey
import javax.inject.Named
import javax.inject.Qualifier

interface Foo

@Qualifier
annotation class Fancy

@Module
abstract class DivergenceModule {
    // Go reports this because `@BindsInstance` contains `@Binds`; FIR is
    // correct to drop it because the function is not a @Binds binding.
    @BindsInstance
    abstract fun seed(foo: Foo): Foo

    // Go reports this because `@BindsOptionalOf` contains `@Binds`; FIR is
    // correct to drop it because the function is not a @Binds binding.
    @BindsOptionalOf
    abstract fun optional(foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the function's qualifier makes the binding alias `Foo` as
    // `@Named("a") Foo`, a different key.
    @Binds
    @Named("a")
    abstract fun qualifiedReturn(foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the parameter's qualifier makes it a different key.
    @Binds
    abstract fun qualifiedParameter(@Named("b") foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the project's own qualifier makes the return a different key.
    @Binds
    @Fancy
    abstract fun customQualifier(foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the qualifiers differ.
    @Binds
    @Named("a")
    abstract fun differentQualifiers(@Named("b") foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the binding contributes Foo to Set<Foo>.
    @Binds
    @IntoSet
    abstract fun intoSet(foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the binding contributes Foo to Map<String, Foo>.
    @Binds
    @IntoMap
    @StringKey("foo")
    abstract fun intoMap(foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the binding contributes the set's elements to Set<Foo>.
    @Binds
    @ElementsIntoSet
    abstract fun elementsIntoSet(foos: Set<Foo>): Set<Foo>

    // Go reports this because it reads the element type `Foo`; FIR is correct
    // to drop it because the parameter's type is Array<out Foo>.
    @Binds
    abstract fun varargs(vararg foo: Foo): Foo

    // Go reports this because it drops `suspend` from the type text; FIR is
    // correct to drop it because a suspend function type is not the plain
    // function type.
    @Binds
    abstract fun suspendType(block: suspend () -> Unit): () -> Unit
}
