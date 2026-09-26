// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 23, 26, 29, 33, 36, 39, 42, 46, 51, 55, 58, 63, 68, 72, 77, 83
// @Binds functions whose single parameter has the return type, the shapes Go
// reports: the finding sits on the function's first line (its modifier list),
// like Go.
package test

import dagger.Binds
import dagger.Module
import javax.inject.Named

interface Foo

class FooImpl : Foo

interface Box<T>

@Module
abstract class PositiveModule {
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun bindFoo(foo: Foo): Foo

    <!BindsReturnTypeMatchesParam!>@Binds<!> abstract fun sameLine(foo: Foo): Foo

    /** KDoc above the annotation does not move the finding. */
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun documented(foo: Foo): Foo

    <!BindsReturnTypeMatchesParam!>@Suppress("unused")<!>
    @Binds
    abstract fun otherAnnotationFirst(foo: Foo): Foo

    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun nullable(foo: Foo?): Foo?

    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun generic(box: Box<Foo>): Box<Foo>

    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun <T> typeParameter(value: T): T

    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun Foo.withReceiver(foo: Foo): Foo

    // The same qualifier on both sides is still the same key.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    @Named("a")
    abstract fun sameQualifier(@Named("a") foo: Foo): Foo

    // Go drops type annotations from the type text; they do not change the key.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun annotatedType(box: @JvmSuppressWildcards Box<Foo>): Box<Foo>

    // A non-qualifier annotation on the parameter does not change the key.
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun annotatedParameter(@Suppress("unused") foo: Foo): Foo

    <!BindsReturnTypeMatchesParam!>@Binds<!>
    abstract fun `backticked name`(foo: Foo): Foo
}

interface PositiveInterfaceModule {
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    fun bindFoo(foo: Foo): Foo
}

// Go does not require an abstract function or a module.
<!BindsReturnTypeMatchesParam!>@Binds<!>
fun topLevel(foo: Foo): Foo = foo

object PositiveObject {
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    fun member(foo: Foo): Foo = foo
}

fun localFunction() {
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    fun local(foo: Foo): Foo = foo
    local(FooImpl())
}

val anonymous = object {
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    fun member(foo: Foo): Foo = foo
}

@Module
abstract class NegativeModule {
    // Implementation to interface: the binding Dagger expects.
    @Binds
    abstract fun bindImpl(impl: FooImpl): Foo

    // No explicit return type: Go reads no return type.
    @Binds
    fun inferred(foo: Foo) = foo

    // Not exactly one value parameter.
    @Binds
    abstract fun Foo.receiverOnly(): Foo

    @Binds
    abstract fun twoParameters(foo: Foo, other: Foo): Foo

    // Nullability differs.
    @Binds
    abstract fun toNullable(foo: Foo): Foo?

    // Type arguments differ.
    @Binds
    abstract fun boxes(box: Box<FooImpl>): Box<Foo>

    // Not @Binds.
    abstract fun notAnnotated(foo: Foo): Foo
}
