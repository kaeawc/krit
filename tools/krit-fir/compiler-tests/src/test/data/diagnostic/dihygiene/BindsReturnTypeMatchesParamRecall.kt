// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// No-op @Binds functions Go misses: Go compares the written type text and
// matches `@Binds` by spelling, while these parameter and return types are the
// same type and the annotation is Dagger's @Binds.
package test

import dagger.Binds
import dagger.Binds as DaggerBinding
import dagger.Module

interface Foo

interface Box<T>

typealias FooAlias = Foo

@Module
abstract class RecallModule {
    // Go misses this because `test.Foo` and `Foo` differ as text.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun qualifiedName(foo: test.Foo): Foo

    // Go misses this because `FooAlias` and `Foo` differ as text.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun typeAlias(foo: FooAlias): Foo

    // Go misses this because the whitespace differs.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun spacing(box: Box< Foo >): Box<Foo>

    // Go misses this because `@dagger.Binds` does not contain `@Binds`.
    <!BindsReturnTypeMatchesParam!>@dagger.Binds<!>
    abstract fun fullyQualified(foo: Foo): Foo

    // Go misses this because `@DaggerBinding` does not contain `@Binds`.
    <!BindsReturnTypeMatchesParam!>@DaggerBinding<!>
    abstract fun importAlias(foo: Foo): Foo

    // Go misses this because it counts the function type's named parameter
    // `x` as a second parameter of the binding.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun functionType(block: (x: Foo) -> Unit): (x: Foo) -> Unit

    // Go misses this because `List<out Foo>` and `List<Foo>` differ as text;
    // List is covariant, so the `out` projection is redundant and the types
    // are the same.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun listOut(foo: List<Foo>): List<out Foo>

    // Go misses this because `Box<*>` and `Box<out Any?>` differ as text; a
    // star projection of an unbounded parameter is `out Any?`, the same type.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun starVsOutAny(foo: Box<*>): Box<out Any?>
}

// Go misses this because it also counts the parameter `x` of the method in the
// default value's object expression, so it sees two parameters.
<!BindsReturnTypeMatchesParam!>@Binds<!>
fun defaultObject(foo: Foo = object : Foo { fun g(x: Int) {} }): Foo = foo

// Go misses this because it reads no type text for the definitely non-null
// type `T & Any`.
<!BindsReturnTypeMatchesParam!>@Binds<!>
fun <T> definitelyNonNull(t: T & Any): T & Any = t
