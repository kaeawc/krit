// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A user annotation class named Binds is not Dagger's or Metro's @Binds.
package test

@Target(AnnotationTarget.FUNCTION)
annotation class Binds

interface Foo

// Neither reports this: the file declares its own Binds and does not import
// Dagger's, so this annotation declares no DI binding.
@Binds
fun lookalike(foo: Foo): Foo = foo
