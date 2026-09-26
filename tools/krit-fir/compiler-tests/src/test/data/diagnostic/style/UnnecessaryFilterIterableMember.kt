// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 21, 23, 25, 50, 64, 72, 80, 89, 105
// A custom filter may return a different type or behave differently from the
// standard-library function. These chains cannot safely suggest a rewrite.
package test

import java.io.File

// A user Iterable with its own filter.
class Files(private val items: List<String>) : Iterable<String> {
    override fun iterator(): Iterator<String> = items.iterator()

    fun filter(predicate: (String) -> Boolean): Files = Files(items.filter(predicate))

    // An implicit receiver.
    fun firstJar(): String = filter { it.endsWith(".jar") }.first()
}

fun files(): Files = Files(listOf("a.jar"))

fun jar(): String = files().filter { it.endsWith(".jar") }.first()

fun jarCount(): Int = files().filter { it.endsWith(".jar") }.count()

fun jarAny(): Boolean = files().filter { it.endsWith(".jar") }.any()

// Recall: Go resolves the parameter's type to Files and skips it.
fun jarParameter(files: Files): String? = files.filter { it.endsWith(".jar") }.lastOrNull()

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
    project.configurations.getByName("runtimeClasspath").filter { it.name.endsWith(".jar") }.first()

// Recall: Go resolves the parameter's type to FileCollection and skips it.
fun collectionJar(collection: FileCollection): Boolean = collection.filter { it.name.endsWith(".jar") }.none()

// A Sequence and a CharSequence with their own filter.
class Lines(private val lines: List<String>) : Sequence<String> {
    override fun iterator(): Iterator<String> = lines.iterator()

    fun filter(predicate: (String) -> Boolean): List<String> = lines.filter(predicate)
}

fun lines(): Lines = Lines(listOf("a"))

fun firstLine(): String = lines().filter { it.isNotEmpty() }.first()

abstract class Text : CharSequence {
    fun filter(predicate: (Char) -> Boolean): String = toString().filter(predicate)
}

fun text(): Text = TODO()

fun digits(): Int = text().filter { it.isDigit() }.count()

// An Iterable object expression and a local class.
val anonymousFiles = object : Iterable<Int> {
    override fun iterator(): Iterator<Int> = listOf(1).iterator()

    fun filter(predicate: (Int) -> Boolean): List<Int> = listOf(1).filter(predicate)

    fun firstPositive(): Int = filter { it > 0 }.first()
}

fun localClass(): Int {
    class Local : Iterable<Int> {
        override fun iterator(): Iterator<Int> = listOf(1).iterator()

        fun filter(predicate: (Int) -> Boolean): List<Int> = listOf(1).filter(predicate)
    }
    return Local().filter { it > 0 }.single()
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
