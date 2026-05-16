package dev.jasonpearson.krit.custom

import org.gradle.testfixtures.ProjectBuilder
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.DataOutputStream
import java.io.File
import java.io.FileOutputStream

class KritRuleServicesTaskTest {

    @TempDir
    lateinit var workspace: File

    @Test
    fun `auto-detects KritRule implementations from compiled classes`() {
        val classesDir = workspace.resolve("classes").apply { mkdirs() }
        val outputDir = workspace.resolve("out")

        writeKritRuleInterfaceClass(classesDir)
        writeClassFile(
            classesDir = classesDir,
            internalName = "com/example/MyRule",
            interfaces = listOf(KRIT_RULE_INTERNAL_NAME),
            isInterface = false,
            isAbstract = false,
        )
        writeClassFile(
            classesDir = classesDir,
            internalName = "com/example/AbstractRule",
            interfaces = listOf(KRIT_RULE_INTERNAL_NAME),
            isInterface = false,
            isAbstract = true,
        )

        val task = newTask(classesDir, resourcesDir = null, outputDir = outputDir)
        task.generate()

        val service = outputDir.resolve("META-INF/services/${KritRuleServicesTask.KRIT_RULE_SERVICE_NAME}")
        assertTrue(service.isFile, "services file should exist")
        val entries = service.readLines().filter { !it.startsWith("#") && it.isNotBlank() }
        assertEquals(listOf("com.example.MyRule"), entries)
    }

    @Test
    fun `preserves manual entries from src resources`() {
        val classesDir = workspace.resolve("classes").apply { mkdirs() }
        val resourcesDir = workspace.resolve("resources").apply { mkdirs() }
        val outputDir = workspace.resolve("out")

        writeKritRuleInterfaceClass(classesDir)
        writeClassFile(
            classesDir = classesDir,
            internalName = "com/example/ScannedRule",
            interfaces = listOf(KRIT_RULE_INTERNAL_NAME),
            isInterface = false,
            isAbstract = false,
        )

        val manual = resourcesDir.resolve("META-INF/services/${KritRuleServicesTask.KRIT_RULE_SERVICE_NAME}")
        manual.parentFile.mkdirs()
        manual.writeText(
            """
            # Pre-existing user entry
            com.example.ManualRule
            """.trimIndent()
        )

        val task = newTask(classesDir, resourcesDir, outputDir)
        task.generate()

        val entries = outputDir.resolve("META-INF/services/${KritRuleServicesTask.KRIT_RULE_SERVICE_NAME}")
            .readLines()
            .filter { !it.startsWith("#") && it.isNotBlank() }
            .toSet()
        assertEquals(setOf("com.example.ManualRule", "com.example.ScannedRule"), entries)
    }

    @Test
    fun `fails with helpful error when no implementations are present`() {
        val classesDir = workspace.resolve("classes").apply { mkdirs() }
        val outputDir = workspace.resolve("out")

        val task = newTask(classesDir, resourcesDir = null, outputDir = outputDir)
        val error = assertThrows(IllegalStateException::class.java) { task.generate() }
        assertTrue(
            error.message!!.contains("No `dev.jasonpearson.krit.api.KritRule`"),
            "error should point users at the missing registration: ${error.message}"
        )
        // Even on failure we leave a placeholder so consumers can read the comment header.
        val service = outputDir.resolve("META-INF/services/${KritRuleServicesTask.KRIT_RULE_SERVICE_NAME}")
        assertTrue(service.isFile, "placeholder services file should exist")
        assertTrue(service.readText().contains("krit-custom-rule-plugin/README"))
    }

    private fun newTask(classesDir: File, resourcesDir: File?, outputDir: File): KritRuleServicesTask {
        val project = ProjectBuilder.builder().withProjectDir(workspace).build()
        val task = project.tasks.register("generateServices", KritRuleServicesTask::class.java).get()
        task.classesDirs.from(classesDir)
        if (resourcesDir != null) {
            task.resourcesDirs.from(resourcesDir)
        }
        task.outputDir.set(outputDir)
        return task
    }

    private fun writeKritRuleInterfaceClass(classesDir: File) {
        writeClassFile(
            classesDir = classesDir,
            internalName = KRIT_RULE_INTERNAL_NAME,
            interfaces = emptyList(),
            isInterface = true,
            isAbstract = true,
        )
    }

    private fun writeClassFile(
        classesDir: File,
        internalName: String,
        interfaces: List<String>,
        isInterface: Boolean,
        isAbstract: Boolean,
        superName: String = "java/lang/Object",
    ) {
        val file = classesDir.resolve("$internalName.class")
        file.parentFile.mkdirs()
        DataOutputStream(FileOutputStream(file)).use { out ->
            out.writeInt(0xCAFEBABE.toInt())
            out.writeShort(0)
            out.writeShort(52)

            // Build constant pool: for each class we need a CONSTANT_Class
            // entry which references a CONSTANT_Utf8 holding the name.
            val pool = mutableListOf<Pair<Int, Any>>() // tag, payload
            val classRefs = mutableMapOf<String, Int>()
            fun addClass(name: String): Int {
                classRefs[name]?.let { return it }
                pool.add(1 to name)
                val utfIndex = pool.size
                pool.add(7 to utfIndex)
                val classIndex = pool.size
                classRefs[name] = classIndex
                return classIndex
            }
            val thisRef = addClass(internalName)
            val superRef = addClass(superName)
            val ifaceRefs = interfaces.map { addClass(it) }

            out.writeShort(pool.size + 1) // constant_pool_count
            for ((tag, payload) in pool) {
                out.writeByte(tag)
                when (tag) {
                    1 -> out.writeUTF(payload as String)
                    7 -> out.writeShort(payload as Int)
                }
            }

            var access = 0x0001 // ACC_PUBLIC
            if (isInterface) access = access or 0x0200
            if (isAbstract) access = access or 0x0400
            out.writeShort(access)
            out.writeShort(thisRef)
            out.writeShort(superRef)
            out.writeShort(ifaceRefs.size)
            for (ref in ifaceRefs) out.writeShort(ref)
            out.writeShort(0) // fields_count
            out.writeShort(0) // methods_count
            out.writeShort(0) // attributes_count
        }
    }

    companion object {
        private const val KRIT_RULE_INTERNAL_NAME = "dev/jasonpearson/krit/api/KritRule"
    }
}
