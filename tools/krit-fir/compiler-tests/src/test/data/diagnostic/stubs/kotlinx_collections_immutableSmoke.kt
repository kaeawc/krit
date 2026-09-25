// Smoke: immutable collection types as Compose-stable parameters.
package stubs

import kotlinx.collections.immutable.ImmutableList
import kotlinx.collections.immutable.ImmutableMap
import kotlinx.collections.immutable.ImmutableSet
import kotlinx.collections.immutable.PersistentList
import kotlinx.collections.immutable.persistentListOf
import kotlinx.collections.immutable.toImmutableList
import kotlinx.collections.immutable.toImmutableMap
import kotlinx.collections.immutable.toImmutableSet
import kotlinx.collections.immutable.toPersistentList

data class ImmutableState(
    val items: ImmutableList<String>,
    val tags: ImmutableSet<String>,
    val counts: ImmutableMap<String, Int>,
)

fun buildState(raw: List<String>): ImmutableState {
    val persistent: PersistentList<String> = persistentListOf("a").add("b")
    val list: ImmutableList<String> = (raw + persistent).toImmutableList()
    val size: Int = list.size + raw.toPersistentList().size
    return ImmutableState(list, raw.toImmutableSet(), mapOf("size" to size).toImmutableMap())
}
