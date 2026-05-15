package dev.krit.intellij

import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiFile
import java.io.File
import java.util.concurrent.TimeUnit

object KritRunner {
    private val log = Logger.getInstance(KritRunner::class.java)

    fun analyze(file: PsiFile): List<KritFinding> {
        val virtualFile = file.virtualFile ?: return emptyList()
        val path = virtualFile.path
        if (!path.endsWith(".kt") && !path.endsWith(".kts")) {
            return emptyList()
        }

        val project = file.project
        val output = File.createTempFile("krit-intellij-", ".json")
        return try {
            val command = mutableListOf(
                kritBinary(),
                "--format=json",
                "-o",
                output.absolutePath,
                "-q",
                path,
            )
            val process = ProcessBuilder(command)
                .directory(project.baseDir())
                .redirectError(ProcessBuilder.Redirect.PIPE)
                .start()

            if (!process.waitFor(30, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                log.warn("krit analysis timed out for $path")
                return emptyList()
            }

            val stderr = process.errorStream.bufferedReader().readText()
            val exit = process.exitValue()
            if (exit != 0 && !output.isFile) {
                log.warn("krit failed for $path with exit $exit: $stderr")
                return emptyList()
            }

            val report = KritJsonParser.parse(output.readText())
            report.findings.filter { File(it.file).canonicalPath == File(path).canonicalPath }
        } catch (t: Throwable) {
            log.warn("krit analysis failed for $path", t)
            emptyList()
        } finally {
            output.delete()
        }
    }

    private fun kritBinary(): String {
        return System.getProperty("krit.binary")
            ?: System.getenv("KRIT_BINARY")
            ?: "krit"
    }

    private fun Project.baseDir(): File? {
        val base = basePath ?: return null
        return File(base)
    }
}

