package dev.jasonpearson.krit.types

import com.intellij.openapi.util.Disposer
import org.jetbrains.kotlin.analysis.api.KaExperimentalApi
import org.jetbrains.kotlin.analysis.api.projectStructure.KaSourceModule
import org.jetbrains.kotlin.psi.KtFile
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.createTempDirectory
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Tag rules for krit-types' source-dependency edges (DepEdges.kt), checked
 * against a real Analysis API session. Each use file resolves declarations
 * from single-purpose files, so an edge's presence and tag identify exactly
 * which position recorded it.
 */
@OptIn(KaExperimentalApi::class)
class DepEdgeScopeTest {
    private data class Edges(val all: Set<String>, val propagating: Set<String>)

    private fun edgesOf(useFile: String): Edges {
        val use = module.psiRoots.filterIsInstance<KtFile>().first { it.name == useFile }
        val tracker = DepTracker()
        assertTrue(analyzeKtFile(use, mutableMapOf(), mutableMapOf(), false, tracker))
        fun names(paths: Set<String>?) = paths.orEmpty().map { Path.of(it).fileName.toString() }.toSet()
        return Edges(
            names(tracker.depPathsByFile[use.virtualFilePath]),
            names(tracker.propagatingDepPathsByFile[use.virtualFilePath]),
        )
    }

    private fun assertPropagating(edges: Edges, file: String) {
        assertTrue(file in edges.propagating, "$file should propagate; edges=$edges")
    }

    private fun assertLocalOnly(edges: Edges, file: String) {
        assertTrue(file in edges.all, "$file should be a direct edge; edges=$edges")
        assertFalse(file in edges.propagating, "$file should not propagate; edges=$edges")
    }

    @Test fun explicitlyTypedBlockBodyDoesNotPropagate() = assertLocalOnly(edgesOf("ExplicitBody.kt"), "BodyTarget.kt")

    @Test fun unitBlockBodyDoesNotPropagate() = assertLocalOnly(edgesOf("UnitBody.kt"), "BodyTarget.kt")

    @Test fun explicitlyTypedExpressionBodyDoesNotPropagate() = assertLocalOnly(edgesOf("ExplicitExpressionBody.kt"), "BodyTarget.kt")

    @Test fun inferredExpressionBodyPropagates() = assertPropagating(edgesOf("InferredBody.kt"), "BodyTarget.kt")

    @Test fun privateInferredBodyPropagates() = assertPropagating(edgesOf("PrivateInferred.kt"), "BodyTarget.kt")

    @Test fun signatureTypesPropagate() {
        val edges = edgesOf("Signature.kt")
        assertPropagating(edges, "SigTarget.kt")
        assertLocalOnly(edges, "BodyTarget.kt")
    }

    @Test fun annotationPropagates() = assertPropagating(edgesOf("Annotated.kt"), "AnnTarget.kt")

    @Test fun parameterDefaultPropagates() = assertPropagating(edgesOf("DefaultValue.kt"), "BodyTarget.kt")

    @Test fun constInitializerPropagates() = assertPropagating(edgesOf("ConstValue.kt"), "ConstTarget.kt")

    @Test fun inlineBodyPropagates() = assertPropagating(edgesOf("InlineBody.kt"), "BodyTarget.kt")

    @Test fun contractBodyPropagates() = assertPropagating(edgesOf("Contract.kt"), "BodyTarget.kt")

    @Test fun inferredPropertyPropagatesAndTypedPropertyDoesNot() {
        assertPropagating(edgesOf("InferredProperty.kt"), "BodyTarget.kt")
        assertLocalOnly(edgesOf("TypedProperty.kt"), "BodyTarget.kt")
    }

    @Test fun getterPropagatesOnlyWhenItDefinesTheType() {
        assertPropagating(edgesOf("InferredGetter.kt"), "BodyTarget.kt")
        assertLocalOnly(edgesOf("TypedGetter.kt"), "BodyTarget.kt")
    }

    @Test fun inferredLocalInsideExplicitBodyDoesNotPropagate() = assertLocalOnly(edgesOf("LocalInferred.kt"), "BodyTarget.kt")

    @Test fun explicitBodyInsideInferredBodyPropagates() = assertPropagating(edgesOf("NestedInInferred.kt"), "BodyTarget.kt")

    @Test fun initializerBlockDoesNotPropagate() = assertLocalOnly(edgesOf("InitBlock.kt"), "BodyTarget.kt")

    @Test fun importedTopLevelFunctionIsAnEdge() {
        val edges = edgesOf("ImportedCall.kt")
        assertLocalOnly(edges, "Imported.kt")
    }

    @Test fun typeAliasAndItsExpansionPropagate() {
        val edges = edgesOf("AliasUse.kt")
        assertPropagating(edges, "Alias.kt")
        assertPropagating(edges, "SigTarget.kt")
    }

    @Test fun typeCheckInsideExplicitBodyDoesNotPropagate() = assertLocalOnly(edgesOf("TypeCheck.kt"), "SigTarget.kt")

    @Test fun inheritedMemberCallReachesItsDeclaringFile() {
        val edges = edgesOf("InheritedCall.kt")
        assertPropagating(edges, "Child.kt")
        assertLocalOnly(edges, "Base.kt")
    }

    @Test fun supertypePropagates() = assertPropagating(edgesOf("Subclass.kt"), "Base.kt")

    @Test fun extensionOperatorWithoutANameIsAnEdge() = assertLocalOnly(edgesOf("OperatorUse.kt"), "Ops.kt")

    @Test fun cacheDepsJsonCarriesPropagatingSubset() {
        val tracker = DepTracker()
        tracker.recordDepPath("/a/Use.kt", "/a/Sig.kt", propagating = true)
        tracker.recordDepPath("/a/Use.kt", "/a/Body.kt", propagating = false)
        val json = buildCacheDepsJson(tracker)
        assertTrue(""""approximation":"kaa-tagged-references"""" in json, json)
        assertTrue(""""depPaths":["/a/Sig.kt","/a/Body.kt"]""" in json, json)
        assertTrue(""""propagatingDepPaths":["/a/Sig.kt"]""" in json, json)
    }

    @Test fun sourceSnapshotDetectsEditsAdditionsAndDeletions() {
        val dir = createTempDirectory("krit-kaa-snapshot-")
        try {
            val a = dir.resolve("A.kt")
            Files.writeString(a, "package p\nfun a() = 1\n")
            val before = snapshotSourceFiles(listOf(dir.toString()))
            assertEquals(before, snapshotSourceFiles(listOf(dir.toString())))
            Files.writeString(a, "package p\nfun a(): Int = 1\n")
            val edited = snapshotSourceFiles(listOf(dir.toString()))
            assertTrue(edited != before)
            Files.writeString(dir.resolve("B.kt"), "package p\n")
            val added = snapshotSourceFiles(listOf(dir.toString()))
            assertTrue(added != edited)
            Files.delete(dir.resolve("B.kt"))
            assertEquals(edited, snapshotSourceFiles(listOf(dir.toString())))
        } finally {
            dir.toFile().deleteRecursively()
        }
    }

    companion object {
        private val sources = mapOf(
            "BodyTarget.kt" to "package p\nfun body(): Int = 1\n",
            "SigTarget.kt" to "package p\nclass Target\n",
            "AnnTarget.kt" to "package p\nannotation class Marker\n",
            "ConstTarget.kt" to "package p\nconst val BASE: Int = 1\n",
            "Imported.kt" to "package lib\nfun imported(): Int = 1\n",
            "Alias.kt" to "package p\ntypealias Alias = Target\n",
            "Base.kt" to "package p\nopen class Base { fun m(): Int = 1 }\n",
            "Child.kt" to "package p\nclass Child : Base()\n",
            "Box.kt" to "package p\nclass Box\n",
            "Ops.kt" to "package p\noperator fun Box.get(i: Int): Int = i\n",

            "ExplicitBody.kt" to "package p\nfun use(): Int { return body() }\n",
            "UnitBody.kt" to "package p\nfun use() { body() }\n",
            "ExplicitExpressionBody.kt" to "package p\nfun use(): Int = body()\n",
            "InferredBody.kt" to "package p\nfun use() = body()\n",
            "PrivateInferred.kt" to "package p\nprivate fun use() = body()\n",
            "Signature.kt" to "package p\nfun use(t: Target): Int = body()\n",
            "Annotated.kt" to "package p\n@Marker fun use(): Int = 1\n",
            "DefaultValue.kt" to "package p\nfun use(x: Int = body()): Int = x\n",
            "ConstValue.kt" to "package p\nconst val DERIVED: Int = BASE + 1\n",
            "InlineBody.kt" to "package p\ninline fun use(): Int { return body() }\n",
            "Contract.kt" to """
                package p
                import kotlin.contracts.ExperimentalContracts
                import kotlin.contracts.contract
                @OptIn(ExperimentalContracts::class)
                fun use(x: Any?): Boolean {
                    contract { returns(true) implies (x != null) }
                    return x != null && body() > 0
                }
            """.trimIndent(),
            "InferredProperty.kt" to "package p\nval use = body()\n",
            "TypedProperty.kt" to "package p\nval use: Int = body()\n",
            "InferredGetter.kt" to "package p\nval use get() = body()\n",
            "TypedGetter.kt" to "package p\nval use: Int get() = body()\n",
            "LocalInferred.kt" to "package p\nfun use(): Int { fun local() = body(); return local() }\n",
            "NestedInInferred.kt" to "package p\nfun use() = object { fun m(): Int { return body() } }.m()\n",
            "InitBlock.kt" to "package p\nclass UseInit { init { body() } }\n",
            "ImportedCall.kt" to "package q\nimport lib.imported\nfun use(): Int { return imported() }\n",
            "AliasUse.kt" to "package q\nimport p.Alias\nfun use(x: Alias): Alias = x\n",
            "TypeCheck.kt" to "package p\nfun use(x: Any): Boolean = x is Target\n",
            "InheritedCall.kt" to "package p\nfun use(c: Child): Int { return c.m() }\n",
            "Subclass.kt" to "package p\nclass UseSub : Base()\n",
            "OperatorUse.kt" to "package p\nfun use(b: Box): Int { return b[0] }\n",
        )

        private val module: KaSourceModule by lazy {
            val root = createTempDirectory("krit-kaa-dep-edges-")
            root.toFile().deleteOnExit()
            for ((name, body) in sources) Files.writeString(root.resolve(name), body)
            val stdlib = Path.of(KotlinVersion::class.java.protectionDomain.codeSource.location.toURI()).toString()
            buildSession(
                Disposer.newDisposable("dep-edge-test"),
                ParsedArgs(sourceDirs = listOf(root.toString()), classpath = listOf(stdlib), jdkHome = null, output = null),
            )
        }
    }
}
