// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses every finding below.
package test

// Go skips any left side whose text contains `?.`; these left sides use a
// safe call only inside an argument, a lambda, a string literal, or an `if`
// condition, and their value does not read through it.
class Item(val tag: String?)

fun argument(items: Map<String?, List<Int>>, item: Item?): List<Int> = <!UseOrEmpty!>items[item?.tag] ?: emptyList()<!>

fun lambda(items: List<Item>, names: Map<Int, String>): String = <!UseOrEmpty!>names[items.indexOfFirst { it.tag?.length == 1 }] ?: ""<!>

fun literal(names: Map<String, String>): String = <!UseOrEmpty!>names["a?.b"] ?: ""<!>

fun condition(item: Item?, name: String?, other: String?): String = <!UseOrEmpty!>(if (item?.tag == null) name else other) ?: ""<!>

// Explicit type arguments on the fallback: tree-sitter parses
// `x ?: emptyList<String>()` as the call `(x ?: emptyList)<String>()`, so Go
// never sees an Elvis with a call on its right.
fun typeArguments(x: List<String>?): List<String> = <!UseOrEmpty!>x ?: emptyList<String>()<!>

fun factoryTypeArguments(x: Set<Int>?): Set<Int> = <!UseOrEmpty!>x ?: setOf<Int>()<!>

fun qualifiedTypeArguments(x: Map<String, Int>?): Map<String, Int> =
    <!UseOrEmpty!>x ?: kotlin.collections.emptyMap<String, Int>()<!>
