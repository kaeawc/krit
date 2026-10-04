// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 37, 42, 47, 52, 57, 62, 67, 76, 82, 88, 94, 100, 106
// Qualifiers compare by their constant argument values, with omitted arguments
// filled in from the annotation's defaults: two qualifiers written differently
// that evaluate to the same arguments are the same key, so the binding is
// still a no-op. Only qualifiers whose values provably differ are a different
// key.
package test

import dagger.Binds
import dagger.Module
import javax.inject.Named
import javax.inject.Qualifier
import kotlin.reflect.KClass

interface Foo

const val KEY = "a"

enum class Color { RED, BLUE }

@Qualifier
annotation class Q(val c: Color)

@Qualifier
annotation class Lvl(val level: Int = 1)

@Qualifier
annotation class Typed(val type: KClass<*>)

@Qualifier
annotation class Tags(vararg val tags: String)

@Module
abstract class SameQualifierModule {
    // A const reference and the literal it holds are the same value.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Named(KEY)
    abstract fun constQualifier(@Named("a") foo: Foo): Foo

    // A simple and a qualified reference to the same enum entry.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Q(Color.RED)
    abstract fun enumQualifier(@Q(test.Color.RED) foo: Foo): Foo

    // An omitted argument takes the annotation's default.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Lvl
    abstract fun defaultArg(@Lvl(level = 1) foo: Foo): Foo

    // javax.inject.Named's value defaults to "".
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Named
    abstract fun namedDefault(@Named("") foo: Foo): Foo

    // A constant concatenation and the string it evaluates to.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Named("a" + "b")
    abstract fun concat(@Named("ab") foo: Foo): Foo

    // The same class literal, spelled simply and qualified.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Typed(Foo::class)
    abstract fun classLiteral(@Typed(test.Foo::class) foo: Foo): Foo

    // The same vararg values, passed positionally and as a named array.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Tags("a", "b")
    abstract fun varargs(@Tags(tags = ["a", "b"]) foo: Foo): Foo
}

@Module
abstract class DifferentQualifierModule {
    // Go reports this because the type text matches; FIR is correct to drop it
    // because the const holds "a", so the qualifiers differ.
    @Binds
    @Named(KEY)
    abstract fun constDiffers(@Named("b") foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the enum entries differ.
    @Binds
    @Q(Color.RED)
    abstract fun enumDiffers(@Q(Color.BLUE) foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the default level 1 differs from 2.
    @Binds
    @Lvl
    abstract fun defaultDiffers(@Lvl(2) foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the default name "" differs from "x".
    @Binds
    @Named
    abstract fun namedDefaultDiffers(@Named("x") foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the class literals differ.
    @Binds
    @Typed(Foo::class)
    abstract fun classDiffers(@Typed(Any::class) foo: Foo): Foo

    // Go reports this because the type text matches; FIR is correct to drop it
    // because the vararg values differ.
    @Binds
    @Tags("a")
    abstract fun varargsDiffer(@Tags("a", "b") foo: Foo): Foo
}
