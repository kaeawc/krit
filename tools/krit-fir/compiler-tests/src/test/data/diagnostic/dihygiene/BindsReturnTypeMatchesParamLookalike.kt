// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13
// A user annotation class named Binds is not Dagger's or Metro's @Binds.
package test

@Target(AnnotationTarget.FUNCTION)
annotation class Binds

interface Foo

// Go reports this because it matches the text `@Binds`; FIR is correct to drop
// it because this annotation declares no DI binding.
@Binds
fun lookalike(foo: Foo): Foo = foo
