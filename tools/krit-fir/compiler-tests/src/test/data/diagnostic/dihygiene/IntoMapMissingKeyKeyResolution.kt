// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 43, 48, 53, 58, 63, 68
// Divergence (precision): Go decides whether a function has a map key from
// the written annotation name (it must end in "Key", and not be "Key" or
// "MapKey"), and it reads the name of a bracketed @[...] group by cutting its
// text at the first "(". So it reports every function below. Each has a map
// key: StringKey under an import alias (provideAliasedKey) or a type alias
// (provideTypeAliasedKey), a project @MapKey annotation whose name does not
// end in Key (provideCustomMapKey, and provideMetaAliasedMapKey, whose @MapKey
// is written through a type alias), a project @MapKey annotation named
// exactly Key (provideKeyNamedKey), and StringKey inside a bracketed group
// that starts with another annotation (provideBracketedKey, where Go reads the
// name "[Named"). The message ("missing a @*Key annotation; Dagger requires a
// key annotation") is not true of them, and Dagger accepts them. No finding
// here.
package test

import dagger.MapKey
import dagger.Provides
import dagger.multibindings.IntoMap
import dagger.multibindings.StringKey
import dagger.multibindings.StringKey as SK
import javax.inject.Named

typealias K = StringKey

typealias MetaMapKey = dagger.MapKey

interface Handler

class HandlerImpl : Handler

@MapKey
annotation class HandlerType(val value: String)

@MetaMapKey
annotation class HandlerKind(val value: String)

@dagger.MapKey
annotation class Key(val value: String)

class KeyResolutionModule {
    @Provides
    @IntoMap
    @SK("aliased")
    fun provideAliasedKey(): Handler = HandlerImpl()

    @Provides
    @IntoMap
    @HandlerType("custom")
    fun provideCustomMapKey(): Handler = HandlerImpl()

    @Provides
    @IntoMap
    @K("typeAliased")
    fun provideTypeAliasedKey(): Handler = HandlerImpl()

    @Provides
    @IntoMap
    @HandlerKind("metaAliased")
    fun provideMetaAliasedMapKey(): Handler = HandlerImpl()

    @Provides
    @IntoMap
    @Key("named")
    fun provideKeyNamedKey(): Handler = HandlerImpl()

    @[Named("x") StringKey("bracketed")]
    @Provides
    @IntoMap
    fun provideBracketedKey(): Handler = HandlerImpl()
}
