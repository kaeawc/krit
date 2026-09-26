// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses every finding below: it matches the annotation text `@IntoSet` /
// `@Provides` and the last dotted segment of the declared type's text, so it
// does not see a collection behind a type alias or parentheses, nor an
// annotation written by its qualified name or through an import alias (an
// import alias of the type is in IntoSetOnNonSetReturnImportAlias.kt). Each
// function is a real Dagger @IntoSet binding that returns a collection
// wrapper, so the message is true of it.
package test

import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoSet
import dagger.Provides as Provider
import dagger.multibindings.IntoSet as IntoMultibinding

interface Plugin

typealias Plugins = List<Plugin>

typealias Registry<T> = HashMap<String, T>

@Module
class PluginModule {
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun typeAlias(): Plugins = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun genericTypeAlias(): Registry<Plugin> = HashMap()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun parenthesized(): (List<Plugin>) = emptyList()

    <!IntoSetOnNonSetReturn!>@dagger.Provides<!>
    @dagger.multibindings.IntoSet
    fun qualifiedAnnotations(): List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provider<!>
    @IntoMultibinding
    fun aliasedAnnotations(): List<Plugin> = emptyList()
}

// Go misses the `@[...]` multi-annotation form: its text match looks for
// `@IntoSet` / `@Provides`, which the modifier text does not contain. These
// are real Dagger @IntoSet bindings of a collection wrapper.
@Module
class MultiAnnotationModule {
    <!IntoSetOnNonSetReturn!>@[Provides IntoSet]<!>
    fun multiAnno(): List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!> @[IntoSet]
    fun multiAnno2(): List<Plugin> = emptyList()
}
