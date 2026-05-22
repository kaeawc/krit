package dev.jasonpearson.krit.intellij

import com.intellij.openapi.application.ReadAction
import com.intellij.openapi.module.ModuleManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.roots.OrderEnumerator
import java.io.File

object KritClasspathResolver {
    // Walks every module's resolved order entries (compile + runtime,
    // recursively into module dependencies) and collects the unique
    // class-output and library-jar paths. Krit's Go side reads CLASSPATH
    // via splitEnvClasspath in oracle_classpath.go, so we pass this
    // through as an env var rather than inventing a new CLI flag.
    //
    // Runs inside a ReadAction because Module / OrderEnumerator access
    // requires it. The walk is in-memory only — no I/O — so it's cheap
    // enough to call per scan without caching.
    fun resolve(project: Project): List<File> {
        return ReadAction.compute<List<File>, RuntimeException> {
            val seen = LinkedHashSet<File>()
            for (module in ModuleManager.getInstance(project).modules) {
                val paths = OrderEnumerator.orderEntries(module)
                    .recursively()
                    .classes()
                    .pathsList
                    .pathList
                for (path in paths) {
                    if (path.isNullOrBlank()) continue
                    val file = File(path)
                    if (file.exists()) {
                        seen.add(file)
                    }
                }
            }
            seen.toList()
        }
    }

    // Joins to the platform classpath form (CLASSPATH env var convention:
    // `:` on Unix, `;` on Windows). Empty list returns empty string, which
    // splitEnvClasspath then treats as no classpath.
    fun toClasspathString(entries: List<File>): String =
        entries.joinToString(File.pathSeparator) { it.absolutePath }
}
