package dev.krit.intellij

import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.project.Project
import java.io.File
import java.util.concurrent.TimeUnit

object KritRunner {
    private val log = Logger.getInstance(KritRunner::class.java)

    fun analyzeProject(project: Project): KritReport {
        val projectDir = project.baseDir() ?: return KritReport()
        val output = File.createTempFile("krit-intellij-", ".json")
        return try {
            val command = mutableListOf(
                kritBinary(),
                "--format=json",
                "-o",
                output.absolutePath,
                "-q",
                projectDir.absolutePath,
            )
            val process = ProcessBuilder(command)
                .directory(projectDir)
                .redirectError(ProcessBuilder.Redirect.PIPE)
                .start()

            if (!process.waitFor(120, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                log.warn("krit project analysis timed out for ${projectDir.path}")
                return KritReport()
            }

            val stderr = process.errorStream.bufferedReader().readText()
            val exit = process.exitValue()
            if (exit != 0 && !output.isFile) {
                log.warn("krit failed for ${projectDir.path} with exit $exit: $stderr")
                return KritReport()
            }

            KritJsonParser.parse(output.readText())
        } catch (t: Throwable) {
            log.warn("krit project analysis failed for ${projectDir.path}", t)
            KritReport()
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
