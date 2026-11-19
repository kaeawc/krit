// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 75, 81, 87
// Functions that are not no-op bindings. Go compares the parameter and return
// type text, skips multibinding contributions, and skips a function whose
// annotations (other than @Binds and non-qualifier ones) differ from its
// parameter's as text, so it leaves most of them alone; the lines it still
// reports are in the go-lines header.
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
    // Neither reports this: the function is not a @Binds binding.
    @BindsInstance
    abstract fun seed(foo: Foo): Foo

    // Neither reports this: the function is not a @Binds binding.
    @BindsOptionalOf
    abstract fun optional(foo: Foo): Foo

    // Neither reports this: the function's qualifier makes the binding alias
    // `Foo` as `@Named("a") Foo`, a different key.
    @Binds
    @Named("a")
    abstract fun qualifiedReturn(foo: Foo): Foo

    // Neither reports this: the parameter's qualifier makes it a different key.
    @Binds
    abstract fun qualifiedParameter(@Named("b") foo: Foo): Foo

    // Neither reports this: the project's own qualifier makes the return a
    // different key.
    @Binds
    @Fancy
    abstract fun customQualifier(foo: Foo): Foo

    // Neither reports this: the qualifiers differ.
    @Binds
    @Named("a")
    abstract fun differentQualifiers(@Named("b") foo: Foo): Foo

    // Neither reports this: the binding contributes Foo to Set<Foo>.
    @Binds
    @IntoSet
    abstract fun intoSet(foo: Foo): Foo

    // Neither reports this: the binding contributes Foo to Map<String, Foo>.
    @Binds
    @IntoMap
    @StringKey("foo")
    abstract fun intoMap(foo: Foo): Foo

    // Neither reports this: the binding contributes the set's elements to Set<Foo>.
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

    // Go reports this because it drops `suspend` from the return type text;
    // FIR is correct to drop it because the plain function type is not a
    // suspend function type.
    @Binds
    abstract fun suspendReturn(block: () -> Unit): suspend () -> Unit

    // Neither reports this: `@N("a")` is `@Named("a")`, so the return is a
    // different key.
    @Binds
    @N("a")
    abstract fun aliasedQualifier(foo: Foo): Foo
}

typealias N = Named
