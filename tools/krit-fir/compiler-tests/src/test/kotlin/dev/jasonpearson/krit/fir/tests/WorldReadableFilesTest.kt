package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// WorldReadableFiles cases a golden cannot express. Most use the platform's
// other MODE_WORLD_READABLE constant, android.os.ParcelFileDescriptor's
// (deprecated since API 19, like Context's), or a project Java class, and the
// shared stubs declare neither; a single-file golden cannot declare a Java
// class, so these sources carry their own Java declarations, with the real
// SDK shapes. Go reports every identifier spelled MODE_WORLD_READABLE that is
// not a property or variable name, so it reports every use and import below,
// the Java lookalike's included. Opening a file with ParcelFileDescriptor's
// world-readable mode is the same exposure, so FIR keeps those; a project
// Java class's own constant of that name is not a platform file mode, so FIR
// does not report it.
class WorldReadableFilesTest {

    private val java = mapOf(
        "android/os/ParcelFileDescriptor.java" to """
            package android.os;

            import java.io.Closeable;
            import java.io.File;
            import java.io.FileNotFoundException;

            public class ParcelFileDescriptor implements Closeable {
                public static final int MODE_READ_ONLY = 268435456;

                @Deprecated
                public static final int MODE_WORLD_READABLE = 1;

                @Deprecated
                public static final int MODE_WORLD_WRITEABLE = 2;

                public static ParcelFileDescriptor open(File file, int mode) throws FileNotFoundException {
                    throw new RuntimeException("Stub!");
                }

                public void close() {
                    throw new RuntimeException("Stub!");
                }
            }
        """.trimIndent(),
        "com/example/legacy/FileModes.java" to """
            package com.example.legacy;

            public final class FileModes {
                public static final int MODE_WORLD_READABLE = 0;
            }
        """.trimIndent(),
    )

    private fun findings(sources: Map<String, String>): List<Pair<String, Int>> {
        val result = KritFirProbe.compile(
            java + sources,
            FirRuleCompileContext(enabledRuleIds = setOf("WorldReadableFiles")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "WorldReadableFiles" }.map { it.file to it.line }
    }

    @Test fun parcelFileDescriptorModeReports() {
        val sources = mapOf(
            "Pfd.kt" to """
                package demo

                import android.os.ParcelFileDescriptor
                import android.os.ParcelFileDescriptor.MODE_WORLD_READABLE
                import java.io.File

                fun qualified(f: File) = ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_WORLD_READABLE)

                fun imported(f: File) = ParcelFileDescriptor.open(f, MODE_WORLD_READABLE)

                fun readOnly(f: File) = ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_READ_ONLY)
            """.trimIndent(),
        )
        assertEquals(listOf("Pfd.kt" to 4, "Pfd.kt" to 7, "Pfd.kt" to 9), findings(sources))
    }

    // Go reports each MODE_WORLD_READABLE identifier once; the golden markers
    // compare lines only, so pin that FIR does not visit a use twice (an
    // annotation argument, a const initializer, a callable reference).
    @Test fun eachUseReportsOnce() {
        val sources = mapOf(
            "Once.kt" to """
                package demo

                import android.content.Context

                annotation class FileMode(val value: Int)

                @FileMode(Context.MODE_WORLD_READABLE)
                fun annotated() = Unit

                const val WORLD = Context.MODE_WORLD_READABLE

                fun reference() = Context::MODE_WORLD_READABLE

                fun branch(mode: Int) = when (mode) {
                    Context.MODE_WORLD_READABLE -> "${'$'}{Context.MODE_WORLD_READABLE}"
                    else -> ""
                }

                fun defaultArg(context: Context, mode: Int = Context.MODE_WORLD_READABLE) =
                    context.getSharedPreferences("data", mode)
            """.trimIndent(),
        )
        assertEquals(
            listOf("Once.kt" to 7, "Once.kt" to 10, "Once.kt" to 12, "Once.kt" to 15, "Once.kt" to 15, "Once.kt" to 19),
            findings(sources).sortedBy { it.second },
        )
    }

    // The goldens compare lines only, so pin the counts behind two declared
    // divergences: a line using both the imported name and an import alias
    // reports each (Go misses the alias, so it reports that line once), and a
    // directive split across lines reports once, on the imported name's line.
    @Test fun aliasUseAndSplitImportCounts() {
        val sources = mapOf(
            "Alias.kt" to """
                package demo

                import android.app.Service.MODE_WORLD_READABLE as SW
                import android.content.Context
                import android.content.Context
                    .MODE_WORLD_READABLE

                fun both(context: Context) = context.getSharedPreferences("data", MODE_WORLD_READABLE or SW)
            """.trimIndent(),
        )
        assertEquals(
            listOf("Alias.kt" to 3, "Alias.kt" to 6, "Alias.kt" to 8, "Alias.kt" to 8),
            findings(sources).sortedBy { it.second },
        )
    }

    @Test fun javaLookalikeConstantDoesNotReport() {
        val sources = mapOf(
            "Lookalike.kt" to """
                package demo

                import com.example.legacy.FileModes
                import com.example.legacy.FileModes.MODE_WORLD_READABLE

                fun lookalike() = FileModes.MODE_WORLD_READABLE

                fun imported() = MODE_WORLD_READABLE
            """.trimIndent(),
        )
        assertEquals(emptyList(), findings(sources))
    }
}
