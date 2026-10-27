package dev.jasonpearson.krit.intellij

import com.intellij.codeHighlighting.HighlightDisplayLevel
import com.intellij.codeInsight.daemon.HighlightDisplayKey
import com.intellij.codeInspection.ex.InspectionProfileImpl
import com.intellij.codeInspection.ex.InspectionToolRegistrar
import com.intellij.lang.annotation.HighlightSeverity
import com.intellij.openapi.components.service
import com.intellij.testFramework.configureInspections
import com.intellij.testFramework.runInInitMode
import com.intellij.testFramework.fixtures.BasePlatformTestCase

// The per-rule inspections end to end: the profile builds one inspection per
// rule from the rule list (DynamicGroupTool), and turning one off or changing
// its level changes what the editor shows. Findings are seeded; krit never
// runs in unit-test mode.
class KritRuleInspectionsIntegrationTest : BasePlatformTestCase() {
    private val fileName = "main.xml"
    // One rule per line: the editor drops a warning that an error covers.
    private val source = "<LinearLayout>\n<TextView/>\n<View/>\n</LinearLayout>\n"
    private val lines = mapOf("UselessParent" to 1, "MagicNumber" to 2)
    private val rules = listOf(
        KritRuleInfo(id = "UselessParent", category = "android", severity = "warning"),
        KritRuleInfo(id = "MagicNumber", category = "style", severity = "warning"),
    )
    override fun setUp() {
        super.setUp()
        KritRules.rulesForTest = rules
        project.service<KritProjectService>().clearFindingsForTest()
        // A fresh current profile per test (the light project is shared);
        // the previous one comes back when the test's disposable goes.
        configureInspections(arrayOf(), project, testRootDisposable)
    }

    override fun tearDown() {
        try {
            KritRules.rulesForTest = null
        } finally {
            super.tearDown()
        }
    }

    private fun finding(rule: String, severity: String = "warning") = KritFinding(
        file = fileName,
        line = lines[rule] ?: 3,
        column = 2,
        ruleSet = "android",
        rule = rule,
        severity = severity,
        message = "message for $rule",
    )

    private fun addRuleInspections(): InspectionProfileImpl {
        val profile = KritProfileRules.profileOf(project)!!
        KritRuleProfiles.install(project, rules)
        return profile
    }

    private fun newProfile() =
        InspectionProfileImpl("krit-test", InspectionToolRegistrar.getInstance(), InspectionProfileImpl.BASE_PROFILE.value)

    private fun highlightsFor(vararg findings: KritFinding): List<com.intellij.codeInsight.daemon.impl.HighlightInfo> {
        myFixture.configureByText(fileName, source)
        project.service<KritProjectService>().seedFindingsForTest(myFixture.file.virtualFile.path, findings.toList())
        val messages = findings.map { it.displayMessage }.toSet()
        return myFixture.doHighlighting().filter { it.description in messages }
    }

    fun `test install adds one inspection per rule under its category`() {
        val profile = addRuleInspections()

        for (rule in rules) {
            val tools = profile.getToolsOrNull(KritRuleFilter.shortNameFor(rule.id), project)
            assertNotNull("expected an inspection for ${rule.id}", tools)
            assertEquals(listOf("Krit", rule.category), tools!!.tool.groupPath.toList())
            assertEquals(rule.id, tools.tool.displayName)
            assertTrue(tools.isEnabled)
            assertEquals(HighlightDisplayLevel.WARNING, tools.defaultState.level)
        }
        // A second install (another project open, a profile switch) adds nothing.
        assertEquals(0, KritRuleProfiles.install(project, rules))
    }

    // runInInitMode: outside it, unit-test profiles skip real initialization
    // and write no tool settings.
    fun `test per-rule settings persist with the profile`() = runInInitMode {
        val saved = newProfile()
        KritRuleProfiles.addMissing(saved, project, rules)
        saved.setToolEnabled(KritRuleFilter.shortNameFor("UselessParent"), false, project)
        saved.setErrorLevel(HighlightDisplayKey.find(KritRuleFilter.shortNameFor("MagicNumber"))!!, HighlightDisplayLevel.ERROR, project)
        // What committing Settings → Inspections does; it also drops the
        // profile's cached set of changed tools that writeScheme reads.
        saved.profileChanged()
        val element = saved.writeScheme()

        // A restart: the profile is read before krit's rule list is known.
        val loaded = newProfile()
        loaded.readExternal(element)
        loaded.initInspectionTools(project)
        assertNull(loaded.getToolsOrNull(KritRuleFilter.shortNameFor("UselessParent"), project))
        KritRuleProfiles.addMissing(loaded, project, rules)

        assertFalse(loaded.getToolsOrNull(KritRuleFilter.shortNameFor("UselessParent"), project)!!.isEnabled)
        val magic = loaded.getToolsOrNull(KritRuleFilter.shortNameFor("MagicNumber"), project)!!
        assertTrue(magic.isEnabled)
        assertEquals(HighlightDisplayLevel.ERROR, magic.defaultState.level)
    }

    fun `test a disabled rule inspection hides only that rule`() {
        val profile = addRuleInspections()
        profile.setToolEnabled(KritRuleFilter.shortNameFor("UselessParent"), false, project)

        val shown = highlightsFor(finding("UselessParent"), finding("MagicNumber"), finding("NotInTheList"))

        val rulesShown = shown.map { it.description }.toSet()
        assertFalse(finding("UselessParent").displayMessage in rulesShown)
        assertTrue(finding("MagicNumber").displayMessage in rulesShown)
        // A rule without a per-rule inspection still shows (through the
        // language inspection and the annotator).
        assertTrue(finding("NotInTheList").displayMessage in rulesShown)
    }

    fun `test a level changed in the profile overrides krit's severity`() {
        val profile = addRuleInspections()
        val key = HighlightDisplayKey.find(KritRuleFilter.shortNameFor("MagicNumber"))!!
        profile.setErrorLevel(key, HighlightDisplayLevel.ERROR, project)

        val raised = highlightsFor(finding("MagicNumber"), finding("UselessParent"))

        val magic = raised.filter { it.description == finding("MagicNumber").displayMessage }
        assertTrue(magic.isNotEmpty())
        assertTrue("expected ERROR, got ${magic.map { it.severity }}", magic.all { it.severity == HighlightSeverity.ERROR })
        val untouched = raised.filter { it.description == finding("UselessParent").displayMessage }
        assertTrue(untouched.isNotEmpty())
        assertTrue(untouched.all { it.severity == HighlightSeverity.WARNING })
    }

    fun `test an untouched level keeps the severity krit reported`() {
        addRuleInspections()

        val shown = highlightsFor(finding("MagicNumber", severity = "error"))

        assertTrue(shown.isNotEmpty())
        assertTrue("got ${shown.map { it.severity }}", shown.all { it.severity == HighlightSeverity.ERROR })
    }
}
