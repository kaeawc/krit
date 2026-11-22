// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8
package test

import kotlin.collections.Map

fun read(items: Map<String, String>): String {
    val result = <!CastNullableToNonNullableType!>items.get("key") as String<!>
    class Map
    return result
}
