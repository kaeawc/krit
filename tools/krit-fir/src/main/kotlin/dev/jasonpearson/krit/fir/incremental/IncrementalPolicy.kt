package dev.jasonpearson.krit.fir.incremental

/**
 * What to do with the whole-compilation-tiered rules ([dev.jasonpearson.krit.fir.FirRule.wholeCompilationTier]) of
 * a request, whose verdicts cannot come from a slice compile.
 */
internal sealed interface TieredDecision {
    /** Serve the request with the full compile, for [reason]. */
    class UseFullCompile(val reason: String) : TieredDecision

    /** Serve it incrementally, running exactly [rules] (a request's "all rules" is never expressed as an empty set). */
    class RunIncrementally(val rules: Set<String>) : TieredDecision
}

/**
 * The seam for deciding how tiered rules are served. [enabled] is every rule the request enables (an empty request
 * set already expanded to all rules), [tiered] the ids of the tiered ones among all rules. A research policy that
 * keeps incremental on for default configurations implements this and is set on the session.
 */
internal fun interface TieredRulePolicy {
    fun decide(enabled: Set<String>, tiered: Set<String>): TieredDecision
}

internal enum class BuiltInTieredPolicy : TieredRulePolicy {
    /** The safe default: any enabled tiered rule means the full compile. */
    FULL_IF_TIERED_ENABLED {
        override fun decide(enabled: Set<String>, tiered: Set<String>): TieredDecision {
            val on = enabled.intersect(tiered)
            return if (on.isEmpty()) TieredDecision.RunIncrementally(enabled)
            else TieredDecision.UseFullCompile("tiered rules enabled: ${on.sorted().joinToString(",")}")
        }
    },

    /**
     * Tests and benchmarks: tiered rules are disabled and every compile of the request, incremental or full,
     * runs only the others. The result is not the request's full verdict.
     */
    EXCLUDE_TIERED {
        override fun decide(enabled: Set<String>, tiered: Set<String>): TieredDecision {
            val rest = enabled - tiered
            return if (rest.isEmpty()) TieredDecision.UseFullCompile("no non-tiered rule enabled")
            else TieredDecision.RunIncrementally(rest)
        }
    },
}

/** Tunables of the planner. */
internal data class PlannerLimits(
    /** More waves than this fall back to the full compile. */
    val maxWaves: Int = 8,
    /**
     * A cumulative dirty set above this fraction of the module falls back to the full compile... A slice costs about
     * 150 ms plus a few ms per compiled file, and the fallback a full compile plus a snapshot, so break-even is
     * well above this. Only files the waves add count: the binary-stale files every slice carries do not.
     */
    val dirtyFraction: Double = 0.25,
    /** ...but never below this many files. */
    val minDirtyFiles: Int = 64,
    /** Binary-stale and removed files kept in every slice: above this fraction of the module (and [staleMinFiles]) the next request re-snapshots. */
    val staleFraction: Double = IncrementalState.DEFAULT_STALE_FRACTION,
    val staleMinFiles: Int = IncrementalState.DEFAULT_STALE_MIN_FILES,
) {
    fun threshold(moduleFiles: Int): Int = maxOf(minDirtyFiles, Math.ceil(moduleFiles * dirtyFraction).toInt())
}

internal enum class PlanPath { INCREMENTAL, FULL }

/** Why a request took the full compile. */
internal enum class FullReason {
    /** No recorded state yet: a normal full check, which builds the snapshot. */
    COLD,

    /** `AnalysisSession.sliceFallback` named a reason: no snapshot, expect/actual, changed compiler facts, a failed slice. */
    SLICE_UNAVAILABLE,
    NEEDS_RESNAPSHOT,
    TIERED_RULES,
    THRESHOLD,
    WAVE_LIMIT,

    /** A wave's compile reported a location-less compiler error, which gates every file of a full compile. */
    GLOBAL_COMPILER_ERROR,

    /**
     * A deleted or re-packaged file left a package with no declaring file in the module. A star import of it
     * records no lookup, and the snapshot keeps the empty package directory, so an importer's error is invisible.
     */
    PACKAGE_REMOVED,

    /** The recorded facts were dropped or fell out of step with the verdict cache. */
    STATE_LOST,

    /** A requested file has no valid verdict and cannot be compiled (it is not a file on disk). */
    MISSING_VERDICT,

    /**
     * SerialVersionUIDInSerializableClass is enabled and a slice moved the set of class names that reach
     * `Serializable` by supertype names, so verdicts of files outside the slice may be wrong.
     */
    SERIALIZABLE_NAMES_CHANGED,
    EXCEPTION,
}

/**
 * What one `check` request came to, for tests and the debug log. [dirtyFiles] is the number of distinct files
 * compiled across the waves; [addedByStaleness] and [addedBySealedClosure] are the files the slices compiled on
 * their own account (binary-stale files and sealed hierarchies); [waveMillis] is each slice compile's wall time.
 * For a full request the counters are those the planner had when it gave up.
 */
internal class PlanTelemetry(
    val path: PlanPath,
    val reason: FullReason?,
    val detail: String?,
    val requestedFiles: Int,
    val changedFiles: Int,
    val newFiles: Int,
    val deletedFiles: Int,
    val unverdictedFiles: Int,
    val dirtyFiles: Int,
    val waves: Int,
    val addedByStaleness: Int,
    val addedBySealedClosure: Int,
    val waveMillis: List<Long>,
    val totalMillis: Long,
) {
    override fun toString(): String =
        "krit-fir incremental: path=$path" + (reason?.let { " reason=$it" } ?: "") + (detail?.let { " ($it)" } ?: "") +
            " requested=$requestedFiles changed=$changedFiles new=$newFiles deleted=$deletedFiles unverdicted=$unverdictedFiles" +
            " dirty=$dirtyFiles waves=$waves staleAdded=$addedByStaleness sealedAdded=$addedBySealedClosure" +
            " waveMs=$waveMillis totalMs=$totalMillis"
}
