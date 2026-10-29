package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.ClassNameFact
import dev.jasonpearson.krit.fir.oracle.AMBIGUOUS_ANNOTATIONS
import dev.jasonpearson.krit.fir.oracle.ClassPayload
import dev.jasonpearson.krit.fir.oracle.DiagnosticPayload
import dev.jasonpearson.krit.fir.oracle.ExpressionPayload
import dev.jasonpearson.krit.fir.oracle.FilePayload
import dev.jasonpearson.krit.fir.oracle.MemberPayload
import dev.jasonpearson.krit.fir.oracle.ParamPayload
import dev.jasonpearson.krit.fir.runner.Finding
import java.nio.charset.StandardCharsets
import java.util.zip.CRC32

/**
 * The persistable part of an [IncrementalState]: everything but what is rebuilt on load (the declaration indexes and
 * the reached-name set) or means nothing in a new process (the surface changes of the latest commit). [snapshot] is
 * the part of the binary snapshot the state tracks; the class files themselves are a directory of the bundle.
 */
internal class PersistedState(
    val compileKey: String,
    val verdictKey: String?,
    val generation: Int,
    val surfaces: Map<String, FileSurface>,
    val contentHashes: Map<String, String>,
    val classFacts: Map<String, List<ClassNameFact>>,
    val lookups: Map<String, Set<Sym>>,
    val snapshot: PersistedSnapshot,
)

/** [outputs] are canonical source -> class files relative to the snapshot's `classes` directory. */
internal class PersistedSnapshot(
    val outputs: Map<String, List<String>>,
    val surfaceHashes: Map<String, String>,
    val binaryStale: Set<String>,
    val removed: Set<String>,
)

/** One file's entry of the verdict cache. */
internal class PersistedVerdict(val file: String, val entry: VerdictCache.Entry)

/**
 * The oracle facts of a bundle (#740): an [OraclePayloadCache] in step with the state at [generation], with the file
 * [order] of its last whole-module compile. A restarted daemon answers the oracle from them instead of compiling the
 * whole module once more.
 */
internal class PersistedOracle(val generation: Int, val order: List<String>, val entries: Map<String, OraclePayloadCache.Entry>)

/**
 * A bundle read back: the state, the verdicts, the oracle facts (null when the writer had none in step) and the name
 * of the class-file directory it was written with.
 */
internal class DecodedBundle(
    val state: PersistedState,
    val verdicts: List<PersistedVerdict>,
    val verdictGeneration: Int,
    val snapshotId: String,
    val oracle: PersistedOracle? = null,
)

/** A bundle that cannot be used: [reason] is for telemetry and tests, never an error for the caller. */
internal class BundleRejected(val reason: Reason, detail: String) : Exception(detail, null, false, false) {
    enum class Reason { MISSING, TRUNCATED, CORRUPT, VERSION, COMPILE_KEY, SNAPSHOT_MISSING, STATE_MISMATCH, IO }
}

/**
 * The compact binary form of a bundle's `bundle.bin`: one file, so a single rename publishes the state and the verdicts
 * together and a reader can never see one without the other.
 *
 * ```
 * magic:int  version:int  compileKey  snapshotId  generation  verdictGeneration  bodyLength:int
 * body   = string table, state, verdicts, oracle facts (version 2)
 * crc32:long  endMagic:int
 * ```
 * Every string of the body (paths, hashes, symbols, messages) is written once into the string table and referenced by
 * index. Integers are varints. The CRC covers everything before it, so a truncated or damaged file is rejected before
 * any of it is interpreted. It is a hand-rolled format on purpose: nothing is deserialised by class name, and a count
 * larger than the bytes left is rejected before anything is allocated for it.
 */
internal object BundleCodec {
    const val VERSION = 2
    private const val MAGIC = 0x4B464942 // "KFIB"
    private const val END_MAGIC = 0x4B454E44 // "KEND"

    /** The ids [newSnapshotId] makes: 32 lower-case hex digits. The id names a directory, so nothing else is accepted. */
    private val SNAPSHOT_ID = Regex("[0-9a-f]{32}")

    fun isSnapshotId(id: String): Boolean = SNAPSHOT_ID.matches(id)

    fun newSnapshotId(): String = java.util.UUID.randomUUID().toString().replace("-", "")

    /** Estimated heap bytes one decoded element (a map entry, a set member, a list slot) costs. */
    private const val ELEMENT_COST = 64L

    /** Estimated heap bytes of a decoded string beyond its characters. */
    private const val STRING_COST = 56L

    /** The share of the heap a decode may be estimated to take; a file that needs more is rejected before it is built. */
    fun defaultDecodeBudget(): Long = Runtime.getRuntime().maxMemory() / 3

    fun encode(
        compileKey: String, snapshotId: String, state: PersistedState, verdicts: Collection<PersistedVerdict>, verdictGeneration: Int,
        oracle: PersistedOracle? = null,
    ): ByteArray {
        require(isSnapshotId(snapshotId)) { "not a snapshot id: $snapshotId" }
        val strings = StringTable()
        val body = Writer()
        writeState(body, strings, state)
        writeVerdicts(body, strings, verdicts)
        writeOracle(body, strings, oracle)
        val table = Writer()
        strings.write(table)
        val bodyBytes = table.size() + body.size()

        val out = Writer(bodyBytes + 256)
        out.int(MAGIC); out.int(VERSION)
        out.string(compileKey); out.string(snapshotId)
        out.varint(state.generation); out.varint(verdictGeneration)
        out.int(bodyBytes)
        out.append(table); out.append(body)
        val crc = CRC32().also { it.update(out.buffer(), 0, out.size()) }.value
        out.long(crc); out.int(END_MAGIC)
        return out.toByteArray()
    }

    /** The header's compile key and version without decoding the body; for cheap rejection and tests. */
    fun peekCompileKey(bytes: ByteArray): String? = try {
        val r = Reader(bytes, bytes.size)
        r.expect(MAGIC); r.int(); r.string()
    } catch (_: Exception) { null }

    /** The snapshot id in the header of a bundle (its first bytes suffice), or null if there is none or it is malformed. The checksum is not checked. */
    fun peekSnapshotId(bytes: ByteArray): String? = try {
        val r = Reader(bytes, bytes.size)
        r.expect(MAGIC); r.int(); r.string()
        r.string().takeIf { isSnapshotId(it) }
    } catch (_: Exception) { null }

    /**
     * Decodes [bytes], which must be a bundle of [compileKey]. Throws [BundleRejected] for anything else: a version
     * this build does not write, another key, a truncated or damaged file.
     */
    fun decode(bytes: ByteArray, compileKey: String, memoryBudget: Long = defaultDecodeBudget()): DecodedBundle {
        try {
            if (bytes.size < 12) throw BundleRejected(BundleRejected.Reason.TRUNCATED, "shorter than a header")
            val head = Reader(bytes, bytes.size)
            if (head.int() != MAGIC) throw BundleRejected(BundleRejected.Reason.CORRUPT, "not a bundle")
            val version = head.int()
            if (version != VERSION) throw BundleRejected(BundleRejected.Reason.VERSION, "format version $version, expected $VERSION")
            if (bytes.size < 12 + 8 + 4) throw BundleRejected(BundleRejected.Reason.TRUNCATED, "no trailer")
            // Integrity first: nothing below is interpreted unless the whole file is what was written.
            val end = bytes.size - 12
            val tail = Reader(bytes, bytes.size).also { it.seek(end) }
            val storedCrc = tail.long()
            if (tail.int() != END_MAGIC) throw BundleRejected(BundleRejected.Reason.TRUNCATED, "no end marker")
            val crc = CRC32().also { it.update(bytes, 0, end) }.value
            if (crc != storedCrc) throw BundleRejected(BundleRejected.Reason.CORRUPT, "checksum mismatch")

            val r = Reader(bytes, end, memoryBudget)
            r.int(); r.int()
            val key = r.string()
            if (key != compileKey) throw BundleRejected(BundleRejected.Reason.COMPILE_KEY, "bundle of another compile key")
            val snapshotId = r.string()
            // The id is a directory name: anything but what the writer makes could point out of the bundle.
            if (!isSnapshotId(snapshotId)) throw BundleRejected(BundleRejected.Reason.CORRUPT, "malformed snapshot id")
            val generation = r.varint()
            val verdictGeneration = r.varint()
            val bodyLength = r.int()
            if (bodyLength != end - r.position()) throw BundleRejected(BundleRejected.Reason.CORRUPT, "body length does not match")
            val strings = readStrings(r)
            val state = readState(r, strings, key, generation)
            val verdicts = readVerdicts(r, strings)
            val oracle = readOracle(r, strings)
            if (r.remaining() != 0) throw BundleRejected(BundleRejected.Reason.CORRUPT, "bytes after the body")
            return DecodedBundle(state, verdicts, verdictGeneration, snapshotId, oracle)
        } catch (e: BundleRejected) {
            throw e
        } catch (_: OutOfMemoryError) {
            // The budget makes this unlikely; if the heap is that full anyway, the bundle is a miss, never a failed check.
            throw BundleRejected(BundleRejected.Reason.IO, "out of memory decoding the bundle")
        } catch (e: Exception) {
            // A damaged length or index that got past the checks: still a rejection, never an error to the caller.
            throw BundleRejected(BundleRejected.Reason.CORRUPT, "${e.javaClass.simpleName}: ${e.message}")
        }
    }

    // ---- state ----------------------------------------------------------------------------------------------------

    private fun writeState(w: Writer, s: StringTable, state: PersistedState) {
        w.varint(s.id(state.verdictKey))
        w.varint(state.contentHashes.size)
        for ((file, hash) in state.contentHashes) { w.varint(s.id(file)); w.varint(s.id(hash)) }

        w.varint(state.surfaces.size)
        for ((file, surface) in state.surfaces) {
            w.varint(s.id(file))
            w.varint(surface.decls.size)
            for ((sym, hash) in surface.decls) { w.varint(s.id(sym.scope)); w.varint(s.id(sym.name)); w.varint(s.id(hash)) }
            w.varint(surface.supers.size)
            for ((cls, supers) in surface.supers) {
                w.varint(s.id(cls)); w.varint(supers.size)
                for (type in supers) w.varint(s.id(type))
            }
            w.varint(surface.sealed.size)
            for (name in surface.sealed) w.varint(s.id(name))
            w.varint(s.id(surface.pkg))
        }

        w.varint(state.classFacts.size)
        for ((file, facts) in state.classFacts) {
            w.varint(s.id(file)); w.varint(facts.size)
            for (fact in facts) {
                w.varint(s.id(fact.name)); w.varint(fact.supertypeNames.size)
                for (name in fact.supertypeNames) w.varint(s.id(name))
            }
        }

        w.varint(state.lookups.size)
        for ((file, syms) in state.lookups) {
            w.varint(s.id(file)); w.varint(syms.size)
            for (sym in syms) { w.varint(s.id(sym.scope)); w.varint(s.id(sym.name)) }
        }

        val snap = state.snapshot
        w.varint(snap.outputs.size)
        for ((source, rels) in snap.outputs) {
            w.varint(s.id(source)); w.varint(rels.size)
            for (rel in rels) w.varint(s.id(rel))
        }
        w.varint(snap.surfaceHashes.size)
        for ((file, hash) in snap.surfaceHashes) { w.varint(s.id(file)); w.varint(s.id(hash)) }
        w.varint(snap.binaryStale.size)
        for (file in snap.binaryStale) w.varint(s.id(file))
        w.varint(snap.removed.size)
        for (file in snap.removed) w.varint(s.id(file))
    }

    private fun readState(r: Reader, strings: Array<String>, compileKey: String, generation: Int): PersistedState {
        fun str() = r.str(strings)
        fun strOrNull() = r.strOrNull(strings)
        val verdictKey = strOrNull()

        val contentHashes = HashMap<String, String>()
        repeat(r.count()) { contentHashes[str()] = str() }

        val surfaces = HashMap<String, FileSurface>()
        repeat(r.count()) {
            val file = str()
            val decls = HashMap<Sym, String>()
            repeat(r.count()) { decls[Sym(str(), str())] = str() }
            val supers = HashMap<String, List<String>>()
            repeat(r.count()) {
                val cls = str()
                val list = ArrayList<String>()
                repeat(r.count()) { list += str() }
                supers[cls] = list
            }
            val sealed = HashSet<String>()
            repeat(r.count()) { sealed += str() }
            surfaces[file] = FileSurface(decls, supers, sealed, str())
        }

        val classFacts = HashMap<String, List<ClassNameFact>>()
        repeat(r.count()) {
            val file = str()
            val facts = ArrayList<ClassNameFact>()
            repeat(r.count()) {
                val name = str()
                val names = HashSet<String>()
                repeat(r.count()) { names += str() }
                facts += ClassNameFact(name, names)
            }
            classFacts[file] = facts
        }

        val lookups = HashMap<String, Set<Sym>>()
        repeat(r.count()) {
            val file = str()
            val syms = HashSet<Sym>()
            repeat(r.count()) { syms += Sym(str(), str()) }
            lookups[file] = syms
        }

        val outputs = HashMap<String, List<String>>()
        repeat(r.count()) {
            val source = str()
            val rels = ArrayList<String>()
            repeat(r.count()) { rels += str() }
            outputs[source] = rels
        }
        val surfaceHashes = HashMap<String, String>()
        repeat(r.count()) { surfaceHashes[str()] = str() }
        val binaryStale = HashSet<String>()
        repeat(r.count()) { binaryStale += str() }
        val removed = HashSet<String>()
        repeat(r.count()) { removed += str() }
        return PersistedState(
            compileKey, verdictKey, generation, surfaces, contentHashes, classFacts, lookups,
            PersistedSnapshot(outputs, surfaceHashes, binaryStale, removed),
        )
    }

    // ---- verdicts -------------------------------------------------------------------------------------------------

    private fun writeVerdicts(w: Writer, s: StringTable, verdicts: Collection<PersistedVerdict>) {
        w.varint(verdicts.size)
        for (v in verdicts) {
            val e = v.entry
            w.varint(s.id(v.file)); w.varint(s.id(e.contentHash)); w.varint(s.id(e.suppliedHash)); w.varint(s.id(e.verdictKey))
            w.varint(s.id(e.verdict.error))
            w.varint(e.verdict.ruleErrors.size)
            for ((rule, message) in e.verdict.ruleErrors) { w.varint(s.id(rule)); w.varint(s.id(message)) }
            w.varint(e.verdict.findings.size)
            for (f in e.verdict.findings) {
                w.varint(s.id(f.path)); w.zigzag(f.line); w.zigzag(f.col); w.zigzag(f.startByte); w.zigzag(f.endByte)
                w.varint(s.id(f.rule)); w.varint(s.id(f.severity)); w.varint(s.id(f.message))
                w.long(java.lang.Double.doubleToRawLongBits(f.confidence))
            }
        }
    }

    private fun readVerdicts(r: Reader, strings: Array<String>): List<PersistedVerdict> {
        fun str() = r.str(strings)
        fun strOrNull() = r.strOrNull(strings)
        val out = ArrayList<PersistedVerdict>()
        repeat(r.count()) {
            val file = str()
            val contentHash = str()
            val supplied = strOrNull()
            val verdictKey = str()
            val error = strOrNull()
            val ruleErrors = LinkedHashMap<String, String>()
            repeat(r.count()) { ruleErrors[str()] = str() }
            val findings = ArrayList<Finding>()
            repeat(r.count()) {
                findings += Finding(
                    path = str(), line = r.zigzag(), col = r.zigzag(), startByte = r.zigzag(), endByte = r.zigzag(),
                    rule = str(), severity = str(), message = str(),
                    confidence = java.lang.Double.longBitsToDouble(r.long()),
                )
            }
            out += PersistedVerdict(file, VerdictCache.Entry(contentHash, supplied, verdictKey, CachedVerdict(findings, error, ruleErrors)))
        }
        return out
    }

    // ---- oracle facts ---------------------------------------------------------------------------------------------

    private fun writeList(w: Writer, s: StringTable, list: List<String>) {
        w.varint(list.size)
        for (item in list) w.varint(s.id(item))
    }

    private fun readList(r: Reader, strings: Array<String>): List<String> {
        val out = ArrayList<String>()
        repeat(r.count()) { out += r.str(strings) }
        return out
    }

    private fun flags(vararg bits: Boolean): Int = bits.foldIndexed(0) { i, acc, on -> if (on) acc or (1 shl i) else acc }

    private fun writeOracle(w: Writer, s: StringTable, oracle: PersistedOracle?) {
        if (oracle == null) { w.varint(0); return }
        w.varint(1)
        w.varint(oracle.generation)
        writeList(w, s, oracle.order)
        w.varint(oracle.entries.size)
        for ((file, e) in oracle.entries) {
            w.varint(s.id(file)); w.varint(s.id(e.contentHash))
            writePayload(w, s, e.payload)
            writeList(w, s, e.depPaths)
            w.varint(e.perFileDeps.size)
            for ((fqn, declaring) in e.perFileDeps) { w.varint(s.id(fqn)); w.varint(s.id(declaring)) }
            w.varint(e.declaredAnnotations.size)
            for ((signature, annotations) in e.declaredAnnotations) {
                w.varint(s.id(signature))
                // The ambiguity marker is compared by identity, so it is written as a flag, not as its contents.
                if (annotations === AMBIGUOUS_ANNOTATIONS) w.varint(1) else { w.varint(0); writeList(w, s, annotations) }
            }
        }
    }

    private fun readOracle(r: Reader, strings: Array<String>): PersistedOracle? {
        fun str() = r.str(strings)
        when (r.varint()) {
            0 -> return null
            1 -> {}
            else -> throw BundleRejected(BundleRejected.Reason.CORRUPT, "bad oracle section marker")
        }
        val generation = r.varint()
        val order = readList(r, strings)
        val entries = LinkedHashMap<String, OraclePayloadCache.Entry>()
        repeat(r.count()) {
            val file = str()
            val hash = str()
            val payload = readPayload(r, strings)
            val depPaths = readList(r, strings)
            val perFileDeps = LinkedHashMap<String, String>()
            repeat(r.count()) { perFileDeps[str()] = str() }
            val declared = LinkedHashMap<String, List<String>>()
            repeat(r.count()) {
                val signature = str()
                declared[signature] = when (r.varint()) {
                    0 -> readList(r, strings)
                    1 -> AMBIGUOUS_ANNOTATIONS
                    else -> throw BundleRejected(BundleRejected.Reason.CORRUPT, "bad annotation marker")
                }
            }
            entries[file] = OraclePayloadCache.Entry(hash, payload, depPaths, perFileDeps, declared)
        }
        return PersistedOracle(generation, order, entries)
    }

    private fun writePayload(w: Writer, s: StringTable, p: FilePayload) {
        w.varint(s.id(p.packageName))
        w.varint(p.declarations.size)
        for (c in p.declarations) writeClass(w, s, c)
        w.varint(p.expressions.size)
        for ((key, e) in p.expressions) {
            w.varint(s.id(key)); w.varint(s.id(e.type))
            w.varint(flags(e.nullable, e.callTargetResolved, e.callTargetSuspend))
            w.zigzag(e.startByte); w.zigzag(e.endByte)
            w.varint(s.id(e.callTarget))
            writeList(w, s, e.annotations)
        }
        w.varint(p.diagnostics.size)
        for (d in p.diagnostics) {
            w.varint(s.id(d.factoryName)); w.varint(s.id(d.severity)); w.varint(s.id(d.message))
            w.zigzag(d.line); w.zigzag(d.col); w.zigzag(d.startByte); w.zigzag(d.endByte)
        }
        w.varint(p.lambdaSuspendByLineCol.size)
        for ((key, suspend) in p.lambdaSuspendByLineCol) { w.varint(s.id(key)); w.varint(if (suspend) 1 else 0) }
    }

    private fun readPayload(r: Reader, strings: Array<String>): FilePayload {
        fun str() = r.str(strings)
        val packageName = str()
        val declarations = ArrayList<ClassPayload>()
        repeat(r.count()) { declarations += readClass(r, strings) }
        val expressions = LinkedHashMap<String, ExpressionPayload>()
        repeat(r.count()) {
            val key = str()
            val type = str()
            val f = r.varint()
            expressions[key] = ExpressionPayload(
                type = type, nullable = f and 1 != 0, startByte = r.zigzag(), endByte = r.zigzag(),
                callTarget = r.strOrNull(strings), callTargetResolved = f and 2 != 0, callTargetSuspend = f and 4 != 0,
                annotations = readList(r, strings),
            )
        }
        val diagnostics = ArrayList<DiagnosticPayload>()
        repeat(r.count()) {
            diagnostics += DiagnosticPayload(
                factoryName = str(), severity = str(), message = str(),
                line = r.zigzag(), col = r.zigzag(), startByte = r.zigzag(), endByte = r.zigzag(),
            )
        }
        val lambdas = LinkedHashMap<String, Boolean>()
        repeat(r.count()) { lambdas[str()] = r.varint() != 0 }
        return FilePayload(packageName, declarations, expressions, diagnostics, lambdas)
    }

    private fun writeClass(w: Writer, s: StringTable, c: ClassPayload) {
        w.varint(s.id(c.fqn)); w.varint(s.id(c.kind))
        writeList(w, s, c.supertypes)
        w.varint(flags(c.isSealed, c.isData, c.isOpen, c.isAbstract))
        w.varint(s.id(c.visibility))
        writeList(w, s, c.typeParameters)
        w.varint(c.members.size)
        for (m in c.members) {
            w.varint(s.id(m.name)); w.varint(s.id(m.kind)); w.varint(s.id(m.returnType))
            w.varint(flags(m.nullable, m.isOverride, m.isAbstract))
            w.varint(s.id(m.visibility))
            w.varint(m.params.size)
            for (p in m.params) { w.varint(s.id(p.name)); w.varint(s.id(p.type)); w.varint(if (p.nullable) 1 else 0) }
            writeList(w, s, m.annotations)
        }
        writeList(w, s, c.annotations)
        w.varint(s.id(c.jarPath))
    }

    private fun readClass(r: Reader, strings: Array<String>): ClassPayload {
        fun str() = r.str(strings)
        val fqn = str()
        val kind = str()
        val supertypes = readList(r, strings)
        val f = r.varint()
        val visibility = str()
        val typeParameters = readList(r, strings)
        val members = ArrayList<MemberPayload>()
        repeat(r.count()) {
            val name = str()
            val memberKind = str()
            val returnType = str()
            val mf = r.varint()
            val memberVisibility = str()
            val params = ArrayList<ParamPayload>()
            repeat(r.count()) { params += ParamPayload(str(), str(), r.varint() != 0) }
            members += MemberPayload(
                name, memberKind, returnType, nullable = mf and 1 != 0, visibility = memberVisibility,
                isOverride = mf and 2 != 0, isAbstract = mf and 4 != 0, params = params, annotations = readList(r, strings),
            )
        }
        val annotations = readList(r, strings)
        return ClassPayload(
            fqn, kind, supertypes, isSealed = f and 1 != 0, isData = f and 2 != 0, isOpen = f and 4 != 0,
            isAbstract = f and 8 != 0, visibility = visibility, typeParameters = typeParameters, members = members,
            annotations = annotations, jarPath = r.strOrNull(strings),
        )
    }

    // ---- strings --------------------------------------------------------------------------------------------------

    // Slot 0 of the string table is the null marker. It is a slot, not a value: no string, whatever its characters,
    // is ever written there, and a reader of a non-null field refuses it.
    private const val NULL_SLOT = 0

    private class StringTable {
        private val ids = HashMap<String, Int>()
        private val order = ArrayList<String>().also { it.add("") } // slot 0: the null marker

        fun id(s: String?): Int {
            if (s == null) return NULL_SLOT
            return ids.getOrPut(s) { order.add(s); order.size - 1 }
        }

        fun write(w: Writer) {
            w.varint(order.size)
            for (s in order) w.bytes(s.toByteArray(StandardCharsets.UTF_8))
        }
    }

    private fun readStrings(r: Reader): Array<String> {
        val n = r.countStrings()
        if (n < 1) throw BundleRejected(BundleRejected.Reason.CORRUPT, "empty string table")
        return Array(n) { r.stringBounded() }
    }

    // ---- bytes ----------------------------------------------------------------------------------------------------

    private class Writer(initial: Int = 1 shl 16) {
        private var buf = ByteArray(maxOf(initial, 16))
        private var len = 0

        fun size() = len
        fun buffer() = buf
        fun toByteArray(): ByteArray = buf.copyOf(len)

        private fun ensure(extra: Int) {
            if (len + extra > buf.size) buf = buf.copyOf(maxOf(buf.size * 2, len + extra))
        }

        fun byte(b: Int) { ensure(1); buf[len++] = b.toByte() }

        fun int(v: Int) { ensure(4); for (shift in intArrayOf(24, 16, 8, 0)) buf[len++] = (v ushr shift).toByte() }

        fun long(v: Long) { ensure(8); for (shift in 56 downTo 0 step 8) buf[len++] = (v ushr shift).toByte() }

        fun varint(value: Int) {
            require(value >= 0) { "negative count or index" }
            var v = value
            ensure(5)
            while (v >= 0x80) { buf[len++] = ((v and 0x7F) or 0x80).toByte(); v = v ushr 7 }
            buf[len++] = v.toByte()
        }

        /** A signed int as a zigzag varint over a long, so every int (negative ones too) round-trips. */
        fun zigzag(v: Int) {
            var z = ((v.toLong() shl 1) xor (v.toLong() shr 63))
            ensure(10)
            while (z and 0x7FL.inv() != 0L) { buf[len++] = ((z and 0x7F) or 0x80).toByte(); z = z ushr 7 }
            buf[len++] = z.toByte()
        }

        fun bytes(b: ByteArray) { varint(b.size); ensure(b.size); System.arraycopy(b, 0, buf, len, b.size); len += b.size }

        fun string(s: String) = bytes(s.toByteArray(StandardCharsets.UTF_8))

        fun append(other: Writer) { ensure(other.len); System.arraycopy(other.buf, 0, buf, len, other.len); len += other.len }
    }

    private class Reader(private val buf: ByteArray, private val limit: Int, private var budget: Long = Long.MAX_VALUE) {
        private var pos = 0

        /** Charges the estimated heap cost of what is about to be built; a file that would take more than the budget is damage. */
        private fun charge(cost: Long) {
            budget -= cost
            if (budget < 0) throw BundleRejected(BundleRejected.Reason.CORRUPT, "too large to decode within the memory budget")
        }

        fun str(strings: Array<String>): String {
            val i = varint()
            if (i == NULL_SLOT || i >= strings.size) throw BundleRejected(BundleRejected.Reason.CORRUPT, "string index out of range")
            return strings[i]
        }

        fun strOrNull(strings: Array<String>): String? {
            val i = varint()
            if (i == NULL_SLOT) return null
            if (i >= strings.size) throw BundleRejected(BundleRejected.Reason.CORRUPT, "string index out of range")
            return strings[i]
        }

        /** The size of the string table: its slots are charged as strings, the reference array as references. */
        fun countStrings(): Int {
            val n = varint()
            if (n < 0 || n > remaining()) throw BundleRejected(BundleRejected.Reason.CORRUPT, "count $n exceeds the bytes left")
            charge(n * 8L)
            return n
        }

        /** One table string, charged for the object and its characters. */
        fun stringBounded(): String {
            val n = varint()
            need(n)
            charge(STRING_COST + n)
            return String(buf, pos, n, StandardCharsets.UTF_8).also { pos += n }
        }

        fun position() = pos
        fun remaining() = limit - pos
        fun seek(to: Int) { pos = to }

        private fun need(n: Int) {
            if (n < 0 || n > limit - pos) throw BundleRejected(BundleRejected.Reason.TRUNCATED, "read past the end")
        }

        fun expect(magic: Int) { if (int() != magic) throw BundleRejected(BundleRejected.Reason.CORRUPT, "bad magic") }

        fun int(): Int { need(4); var v = 0; repeat(4) { v = (v shl 8) or (buf[pos++].toInt() and 0xFF) }; return v }

        fun long(): Long { need(8); var v = 0L; repeat(8) { v = (v shl 8) or (buf[pos++].toLong() and 0xFF) }; return v }

        fun varint(): Int {
            var shift = 0
            var v = 0
            while (true) {
                need(1)
                val b = buf[pos++].toInt()
                v = v or ((b and 0x7F) shl shift)
                if (b and 0x80 == 0) return v
                shift += 7
                if (shift > 28) throw BundleRejected(BundleRejected.Reason.CORRUPT, "varint too long")
            }
        }

        fun zigzag(): Int {
            var shift = 0
            var z = 0L
            while (true) {
                need(1)
                val b = buf[pos++].toInt()
                z = z or ((b and 0x7F).toLong() shl shift)
                if (b and 0x80 == 0) break
                shift += 7
                if (shift > 63) throw BundleRejected(BundleRejected.Reason.CORRUPT, "varint too long")
            }
            val v = (z ushr 1) xor -(z and 1)
            if (v < Int.MIN_VALUE || v > Int.MAX_VALUE) throw BundleRejected(BundleRejected.Reason.CORRUPT, "int out of range")
            return v.toInt()
        }

        /** A count of elements that each take at least one byte: more than the bytes left is damage, not data. */
        fun count(): Int {
            val n = varint()
            if (n < 0 || n > remaining()) throw BundleRejected(BundleRejected.Reason.CORRUPT, "count $n exceeds the bytes left")
            charge(n * ELEMENT_COST)
            return n
        }

        fun bytes(): ByteArray {
            val n = varint()
            need(n)
            return buf.copyOfRange(pos, pos + n).also { pos += n }
        }

        fun string(): String = String(bytes(), StandardCharsets.UTF_8)
    }
}
