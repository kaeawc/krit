// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15
// Metro's @Binds declares a binding too.
package test

import dev.zacsweers.metro.BindingContainer
import dev.zacsweers.metro.Binds

interface Foo

class FooImpl : Foo

@BindingContainer
interface MetroBindings {
    <!BindsReturnTypeMatchesParam!>@Binds<!>
    fun bindFoo(foo: Foo): Foo

    // Metro's receiver form has no value parameter.
    @Binds
    fun Foo.bindReceiver(): Foo

    @Binds
    fun bindImpl(impl: FooImpl): Foo
}
