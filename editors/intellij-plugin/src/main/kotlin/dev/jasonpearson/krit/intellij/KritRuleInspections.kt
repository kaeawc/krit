package dev.jasonpearson.krit.intellij

import com.intellij.codeHighlighting.HighlightDisplayLevel
import com.intellij.codeInsight.daemon.HighlightDisplayKey
import com.intellij.codeInspection.LocalInspectionTool
import com.intellij.codeInspection.ProblemHighlightType
import com.intellij.codeInspection.ProblemsHolder
import com.intellij.codeInspection.ex.InspectionProfileImpl
import com.intellij.codeInspection.ex.LocalInspectionToolWrapper
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.application.PathManager
import com.intellij.openapi.components.service
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.progress.ProcessCanceledException
import com.intellij.openapi.project.Project
import com.intellij.profile.codeInspection.InspectionProfileManager
import com.intellij.profile.codeInspection.InspectionProjectProfileManager
import com.intellij.profile.codeInspection.ProjectInspectionProfileManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiElementVisitor
import com.intellij.psi.PsiFile
import java.io.File

/**
 * The krit rule list behind the per-rule inspections: what the project's krit
 * binary prints for `--list-rules --format json`, through [KritRuleCatalog]'s
 * per-binary cache (memory, then a file under the IDE system directory).
 */
object KritRules {
    private val log = Logger.getInstance(KritRules::class.java)

    private val catalog: KritRuleCatalog by lazy {
        KritRuleCatalog(File(PathManager.getSystemPath(), "krit/rule-lists"))
    }

    // Unit tests never run krit; they seed the list here instead.
    @Volatile
    internal var rulesForTest: List<KritRuleInfo>? = null

    /**
     * The rules of [project]'s krit binary, running it for them when they
     * aren't cached (once per binary; later calls and IDE restarts read the
     * cache). Empty when no binary resolves or it can't list rules (a krit
     * older than `--list-rules --format json`). Blocks: call off the EDT.
     */
    fun forProject(project: Project): List<KritRuleInfo> {
        if (ApplicationManager.getApplication()?.isUnitTestMode == true) return rulesForTest.orEmpty()
        val binary = KritBinaryResolver.find(project) ?: return emptyList()
        return try {
            catalog.rulesFor(binary).orEmpty()
        } catch (e: ProcessCanceledException) {
            throw e
        } catch (t: RuntimeException) {
            log.warn("krit rule list unavailable from ${binary.path}", t)
            emptyList()
        }
    }

    /** Drops the cached list of [project]'s binary and fetches it again. Blocks. */
    fun refresh(project: Project): List<KritRuleInfo>? {
        if (ApplicationManager.getApplication()?.isUnitTestMode == true) return rulesForTest
        val binary = KritBinaryResolver.find(project) ?: return null
        return catalog.refresh(binary)
    }
}

/**
 * One krit rule as an inspection (`Krit_<RuleId>`), under
 * Settings → Inspections → Krit → <category>. It reports that rule's
 * findings; see [KritRuleFilter] for how its enabled state and level combine
 * with krit.yml.
 */
class KritRuleInspection(val rule: KritRuleInfo) : LocalInspectionTool() {
    // Every rule is an inspection, so most have nothing in a given file;
    // skip those files without walking them.
    override fun isAvailableForFile(file: PsiFile): Boolean = findingsIn(file).isNotEmpty()

    override fun buildVisitor(holder: ProblemsHolder, isOnTheFly: Boolean): PsiElementVisitor {
        if (findingsIn(holder.file).isEmpty()) return PsiElementVisitor.EMPTY_VISITOR
        return object : PsiElementVisitor() {
            override fun visitElement(element: PsiElement) {
                if (element.parent != null) return
                val file = element.containingFile ?: return
                val findings = findingsIn(file)
                if (findings.isEmpty()) return
                val customized = KritProfileRules.settingFor(file, rule.id)?.customized == true
                for (finding in findings) {
                    val range = KritRanges.rangeFor(file, finding)
                    val target = file.findElementAt(range.startOffset) ?: file
                    // A level changed in the profile applies through
                    // GENERIC_ERROR_OR_WARNING (scopes included); otherwise
                    // show the severity krit reported.
                    val type = if (customized) {
                        ProblemHighlightType.GENERIC_ERROR_OR_WARNING
                    } else {
                        KritSeverity.problemHighlightType(KritRuleFilter.reportedLevel(finding))
                    }
                    holder.registerProblem(target, finding.displayMessage, type, *KritQuickFixes.forFinding(finding))
                }
            }
        }
    }

    private fun findingsIn(file: PsiFile): List<KritFinding> {
        val path = file.virtualFile?.path ?: return emptyList()
        if (file.project.isDisposed) return emptyList()
        return file.project.service<KritProjectService>().findingsFor(path).filter { it.rule == rule.id }
    }

    override fun getShortName(): String = KritRuleFilter.shortNameFor(rule.id)

    override fun getDisplayName(): String = rule.id

    override fun getGroupDisplayName(): String = rule.category

    override fun getGroupPath(): Array<String> = arrayOf("Krit", rule.category)

    override fun getDefaultLevel(): HighlightDisplayLevel =
        KritSeverity.displayLevel(KritRuleFilter.defaultLevel(rule.severity))

    override fun isEnabledByDefault(): Boolean = true

    override fun getStaticDescription(): String {
        val text = rule.description?.takeIf { it.isNotBlank() } ?: "Krit rule ${rule.id}."
        return "<html><body>${escape(text)}<p>Reported by the krit analyzer " +
            "(${escape(rule.category)}, ${escape(rule.severity)}). krit.yml decides whether the rule runs; " +
            "turning this inspection off only hides its findings.</p></body></html>"
    }

    private fun escape(s: String): String =
        s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
}

/**
 * Wrapper for a [KritRuleInspection]. The profile copies wrappers, and the
 * platform's copy re-instantiates the tool class through a no-arg
 * constructor, which a per-rule inspection doesn't have.
 */
class KritRuleToolWrapper(private val ruleTool: KritRuleInspection) : LocalInspectionToolWrapper(ruleTool) {
    override fun createCopy(): LocalInspectionToolWrapper = KritRuleToolWrapper(KritRuleInspection(ruleTool.rule))
}

/**
 * Puts the per-rule inspections into inspection profiles. The platform builds
 * a profile's tools from plugin.xml, which can't list rules only the krit
 * binary knows, so the plugin adds them with [InspectionProfileImpl.addTool]
 * once the rule list is known. addTool also restores the settings the
 * profile stored for the tool (enabled state, level, scopes), so choices made
 * in Settings → Inspections survive restarts, and the Settings copy of a
 * profile is built from its tools, so the rules show up there.
 */
object KritRuleProfiles {
    /**
     * Adds the inspections for [rules] to every inspection profile [project]
     * can use (its project profiles and the IDE profiles) that lacks them,
     * and returns how many it added to the project's current profile. Runs
     * on the EDT.
     */
    fun install(project: Project, rules: List<KritRuleInfo>): Int {
        if (rules.isEmpty() || project.isDisposed) return 0
        val current = KritProfileRules.profileOf(project)
        val profiles = LinkedHashSet<InspectionProfileImpl>()
        current?.let(profiles::add)
        profiles += ProjectInspectionProfileManager.getInstance(project).profiles
        profiles += InspectionProfileManager.getInstance().profiles
        var addedToCurrent = 0
        for (profile in profiles) {
            val added = addMissing(profile, project, rules)
            if (profile === current) addedToCurrent = added
        }
        return addedToCurrent
    }

    /** Adds an inspection to [profile] for each of [rules] it doesn't have; returns how many. */
    fun addMissing(profile: InspectionProfileImpl, project: Project, rules: List<KritRuleInfo>): Int {
        profile.initInspectionTools(project)
        var added = 0
        for (rule in rules) {
            if (profile.getToolsOrNull(KritRuleFilter.shortNameFor(rule.id), project) != null) continue
            profile.addTool(project, KritRuleToolWrapper(KritRuleInspection(rule)), HashMap())
            added++
        }
        if (added > 0) profile.profileChanged()
        return added
    }
}

/** Reads the per-rule inspections' state from a file's inspection profile. */
object KritProfileRules {
    /**
     * The profile's state for [ruleId]'s inspection at [file], or null when
     * the profile has no such inspection (no rule list, or a rule newer than
     * the list).
     */
    fun settingFor(file: PsiFile, ruleId: String): KritRuleFilter.RuleSetting? =
        settingFor(profileOf(file.project) ?: return null, file, ruleId)

    fun settingFor(profile: InspectionProfileImpl, file: PsiFile, ruleId: String): KritRuleFilter.RuleSetting? {
        val shortName = KritRuleFilter.shortNameFor(ruleId)
        val tools = profile.getToolsOrNull(shortName, file.project) ?: return null
        val key = HighlightDisplayKey.find(shortName) ?: return null
        val enabled = profile.isToolEnabled(key, file)
        val level = profile.getErrorLevel(key, file)
        val customized = level != tools.tool.defaultLevel
        return KritRuleFilter.RuleSetting(enabled, KritSeverity.levelOf(level), customized)
    }

    /** Whether the profile has a per-rule inspection for [ruleId]. */
    fun hasRuleInspection(project: Project, ruleId: String): Boolean =
        profileOf(project)?.getToolsOrNull(KritRuleFilter.shortNameFor(ruleId), project) != null

    fun profileOf(project: Project): InspectionProfileImpl? = try {
        InspectionProjectProfileManager.getInstance(project).currentProfile
    } catch (e: ProcessCanceledException) {
        throw e
    } catch (_: RuntimeException) {
        null
    }
}

internal object KritSeverity {
    fun highlightSeverity(level: KritLevel): HighlightSeverity = when (level) {
        KritLevel.ERROR -> HighlightSeverity.ERROR
        KritLevel.WARNING -> HighlightSeverity.WARNING
        KritLevel.WEAK_WARNING -> HighlightSeverity.WEAK_WARNING
        KritLevel.INFORMATION -> HighlightSeverity.INFORMATION
    }

    fun problemHighlightType(level: KritLevel): ProblemHighlightType = when (level) {
        KritLevel.ERROR -> ProblemHighlightType.ERROR
        KritLevel.WARNING -> ProblemHighlightType.WARNING
        KritLevel.WEAK_WARNING -> ProblemHighlightType.WEAK_WARNING
        KritLevel.INFORMATION -> ProblemHighlightType.INFORMATION
    }

    fun highlightSeverity(finding: KritFinding): HighlightSeverity =
        highlightSeverity(KritRuleFilter.reportedLevel(finding))

    fun problemHighlightType(finding: KritFinding): ProblemHighlightType =
        problemHighlightType(KritRuleFilter.reportedLevel(finding))

    // The inspection level a rule's registry severity defaults to. "info"
    // findings render as INFORMATION (no highlighting, fixes still offered),
    // which is the profile's "No highlighting, only fix" level.
    fun displayLevel(level: KritLevel): HighlightDisplayLevel = when (level) {
        KritLevel.ERROR -> HighlightDisplayLevel.ERROR
        KritLevel.WARNING -> HighlightDisplayLevel.WARNING
        KritLevel.WEAK_WARNING -> HighlightDisplayLevel.WEAK_WARNING
        KritLevel.INFORMATION -> HighlightDisplayLevel.DO_NOT_SHOW
    }

    // A profile level (possibly a custom severity) as a plugin level, by
    // where its severity falls among the standard ones.
    fun levelOf(level: HighlightDisplayLevel): KritLevel {
        val severity = level.severity
        return when {
            severity >= HighlightSeverity.ERROR -> KritLevel.ERROR
            severity >= HighlightSeverity.WARNING -> KritLevel.WARNING
            severity >= HighlightSeverity.WEAK_WARNING -> KritLevel.WEAK_WARNING
            else -> KritLevel.INFORMATION
        }
    }
}
