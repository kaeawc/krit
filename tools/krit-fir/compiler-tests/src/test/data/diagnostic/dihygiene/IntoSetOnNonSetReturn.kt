// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 23, 27, 29, 33, 37, 41, 45, 49, 53, 57, 61, 65, 69, 73, 77, 81, 85, 89, 93, 99, 103, 107, 114, 119, 123, 127, 131, 135, 139, 143, 148, 156, 163, 170, 175, 180, 185, 192, 203, 207
// Positive: an @IntoSet @Provides / @Binds function whose declared return type
// is a collection wrapper. Each is reported on the function's first line (its
// modifier list), the line the Go rule reports. Every case here is also a Go
// finding.
package test

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoSet

interface Plugin
class PluginImpl : Plugin

@Module
class PluginModule {
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun providePluginList(): List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@IntoSet<!>
    @Provides
    fun reversedOrder(): MutableList<Plugin> = mutableListOf()

    <!IntoSetOnNonSetReturn!>@Provides<!> @IntoSet fun oneLine(): ArrayList<Plugin> = ArrayList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun set(): Set<Plugin> = emptySet()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun mutableSet(): MutableSet<Plugin> = mutableSetOf()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun hashSet(): HashSet<Plugin> = HashSet()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun linkedHashSet(): LinkedHashSet<Plugin> = LinkedHashSet()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun map(): Map<String, Plugin> = emptyMap()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun mutableMap(): MutableMap<String, Plugin> = mutableMapOf()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun hashMap(): HashMap<String, Plugin> = HashMap()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun linkedHashMap(): LinkedHashMap<String, Plugin> = LinkedHashMap()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun collection(): Collection<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun mutableCollection(): MutableCollection<Plugin> = mutableListOf()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun iterable(): Iterable<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun mutableIterable(): MutableIterable<Plugin> = mutableListOf()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun array(): Array<Plugin> = arrayOf(PluginImpl())

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun nullable(): List<Plugin>? = null

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun qualified(): kotlin.collections.List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun javaQualified(): java.util.ArrayList<Plugin> = java.util.ArrayList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun block(): List<Plugin> {
        return listOf(PluginImpl())
    }

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun withParameter(plugin: Plugin): List<Plugin> = listOf(plugin)

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun starProjected(): List<*> = emptyList<Plugin>()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun nested(): List<List<Plugin>> = emptyList()

    /**
     * KDoc is not part of the reported line.
     */
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun documented(): List<Plugin> = emptyList()

    // A line comment is not part of the reported line either.
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun commented(): List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun `backticked name`(): List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>internal<!> @Provides
    @IntoSet
    fun internalModifier(): List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun Plugin.extension(): List<Plugin> = listOf(this)

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun <T : Plugin> generic(value: T): List<T> = listOf(value)

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun annotatedType(): @JvmSuppressWildcards List<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun annotatedArgument(): List<@JvmSuppressWildcards Plugin> = emptyList()

    companion object {
        <!IntoSetOnNonSetReturn!>@Provides<!>
        @IntoSet
        fun fromCompanion(): List<Plugin> = emptyList()
    }
}

@Module
abstract class BindsModule {
    <!IntoSetOnNonSetReturn!>@Binds<!>
    @IntoSet
    abstract fun bindList(impl: ArrayList<Plugin>): List<Plugin>
}

@Module
interface BindsInterfaceModule {
    <!IntoSetOnNonSetReturn!>@Binds<!>
    @IntoSet
    fun bindSet(impl: HashSet<Plugin>): Set<Plugin>
}

@Module
object ObjectModule {
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun fromObject(): List<Plugin> = emptyList()
}

<!IntoSetOnNonSetReturn!>@Provides<!>
@IntoSet
fun topLevel(): List<Plugin> = emptyList()

fun host(): Plugin {
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun local(): List<Plugin> = emptyList()

    val anonymous = object {
        <!IntoSetOnNonSetReturn!>@Provides<!>
        @IntoSet
        fun member(): List<Plugin> = emptyList()
    }
    anonymous.member()

    class Local {
        <!IntoSetOnNonSetReturn!>@Provides<!>
        @IntoSet
        fun member(): List<Plugin> = emptyList()
    }
    Local().member()
    return local().first()
}

@Module
class JavaCollectionModule {
    // The Java interfaces Kotlin maps to its own collections, written out.
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun javaList(): java.util.List<Plugin> = java.util.ArrayList<Plugin>() as java.util.List<Plugin>

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun javaIterable(): java.lang.Iterable<Plugin> = java.util.ArrayList<Plugin>() as java.lang.Iterable<Plugin>
}
