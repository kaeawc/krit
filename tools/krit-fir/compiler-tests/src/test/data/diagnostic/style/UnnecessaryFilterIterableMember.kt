// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 25, 27, 29, 54, 68, 76, 84, 93, 109
// A `filter` that is not the stdlib function, called on an Iterable (or a
// Sequence or CharSequence), followed by a stdlib terminal. The stdlib
// `.first { pred }` the message suggests exists on the receiver and does the
// same thing, so these are reported like the stdlib filter. Go matches the
// names and reports each chain whose receiver it cannot resolve to a type
// outside its list.
package test

import java.io.File

// A user Iterable with its own filter.
class Files(private val items: List<String>) : Iterable<String> {
    override fun iterator(): Iterator<String> = items.iterator()

    fun filter(predicate: (String) -> Boolean): Files = Files(items.filter(predicate))

    // An implicit receiver.
    fun firstJar(): String = <!UnnecessaryFilter!>filter<!> { it.endsWith(".jar") }.first()
}

fun files(): Files = Files(listOf("a.jar"))

fun jar(): String = <!UnnecessaryFilter!>files<!>().filter { it.endsWith(".jar") }.first()

fun jarCount(): Int = <!UnnecessaryFilter!>files<!>().filter { it.endsWith(".jar") }.count()

fun jarAny(): Boolean = <!UnnecessaryFilter!>files<!>().filter { it.endsWith(".jar") }.any()

// Recall: Go resolves the parameter's type to Files and skips it.
fun jarParameter(files: Files): String? = <!UnnecessaryFilter!>files<!>.filter { it.endsWith(".jar") }.lastOrNull()

// A filter that takes a SAM interface, like Gradle's FileCollection.filter(Spec).
fun interface Spec<T> {
    fun isSatisfiedBy(element: T): Boolean
}

interface FileCollection : Iterable<File> {
    fun filter(spec: Spec<in File>): FileCollection
}

interface Configuration : FileCollection

interface ConfigurationContainer {
    fun getByName(name: String): Configuration
}

interface Project {
    val configurations: ConfigurationContainer
}

fun runtimeJar(project: Project): File =
    <!UnnecessaryFilter!>project<!>.configurations.getByName("runtimeClasspath").filter { it.name.endsWith(".jar") }.first()

// Recall: Go resolves the parameter's type to FileCollection and skips it.
fun collectionJar(collection: FileCollection): Boolean = <!UnnecessaryFilter!>collection<!>.filter { it.name.endsWith(".jar") }.none()

// A Sequence and a CharSequence with their own filter.
class Lines(private val lines: List<String>) : Sequence<String> {
    override fun iterator(): Iterator<String> = lines.iterator()

    fun filter(predicate: (String) -> Boolean): List<String> = lines.filter(predicate)
}

fun lines(): Lines = Lines(listOf("a"))

fun firstLine(): String = <!UnnecessaryFilter!>lines<!>().filter { it.isNotEmpty() }.first()

abstract class Text : CharSequence {
    fun filter(predicate: (Char) -> Boolean): String = toString().filter(predicate)
}

fun text(): Text = TODO()

fun digits(): Int = <!UnnecessaryFilter!>text<!>().filter { it.isDigit() }.count()

// An Iterable object expression and a local class.
val anonymousFiles = object : Iterable<Int> {
    override fun iterator(): Iterator<Int> = listOf(1).iterator()

    fun filter(predicate: (Int) -> Boolean): List<Int> = listOf(1).filter(predicate)

    fun firstPositive(): Int = <!UnnecessaryFilter!>filter<!> { it > 0 }.first()
}

fun localClass(): Int {
    class Local : Iterable<Int> {
        override fun iterator(): Iterator<Int> = listOf(1).iterator()

        fun filter(predicate: (Int) -> Boolean): List<Int> = listOf(1).filter(predicate)
    }
    return <!UnnecessaryFilter!>Local<!>().filter { it > 0 }.single()
}

// Precision: a member filter on an Iterable followed by a member terminal. The
// terminal is not the stdlib function, so `.first { }` would call the stdlib
// one instead of Rows.first(). Go matches the names and reports it.
class Rows(private val rows: List<String>) : Iterable<String> {
    override fun iterator(): Iterator<String> = rows.iterator()

    fun filter(predicate: (String) -> Boolean): Rows = this

    fun first(): String = rows[0]
}

fun rows(): Rows = Rows(listOf("a"))

fun firstRow(): String = rows().filter { it.isNotEmpty() }.first()

// Negative: the predicate is not a trailing lambda.
fun jarReference(): String = files().filter(::isJar).first()

fun isJar(name: String): Boolean = name.endsWith(".jar")
