package dev.jasonpearson.krit.intellij

import com.intellij.codeInsight.intention.IntentionAction
import com.intellij.codeInspection.LocalQuickFix
import com.intellij.codeInspection.ProblemDescriptor
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.JavaPsiFacade
import com.intellij.psi.PsiAnnotation
import com.intellij.psi.PsiArrayInitializerMemberValue
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.PsiLiteralExpression
import com.intellij.psi.PsiModifierListOwner
import com.intellij.psi.util.PsiTreeUtil
import org.jetbrains.kotlin.psi.KtAnnotationEntry
import org.jetbrains.kotlin.psi.KtModifierListOwner
import org.jetbrains.kotlin.psi.KtPsiFactory

internal object KritSuppress {
    const val SUPPRESS_FAMILY = "Krit suppress"

    fun titleFor(ruleId: String): String = "Suppress Krit '$ruleId' on this declaration"

    // Pure helper for merging args: dedupes the rule id into an existing
    // argument list. Returns null if the rule id is already present
    // (caller skips the edit so it remains a no-op).
    fun mergeArguments(existing: List<String>, ruleId: String): List<String>? {
        if (existing.contains(ruleId)) return null
        return existing + ruleId
    }

    fun formatJavaSuppressValue(args: List<String>): String {
        if (args.size == 1) return "\"${args.single()}\""
        return "{${args.joinToString(", ") { "\"$it\"" }}}"
    }

    fun applyToElement(element: PsiElement, ruleId: String) {
        val owner = enclosingOwner(element) ?: return
        when (owner) {
            is KtModifierListOwner -> applyKotlin(owner, ruleId)
            is PsiModifierListOwner -> applyJava(owner, ruleId)
        }
    }

    fun enclosingOwner(element: PsiElement): PsiElement? {
        return PsiTreeUtil.getParentOfType(element, KtModifierListOwner::class.java)
            ?: PsiTreeUtil.getParentOfType(element, PsiModifierListOwner::class.java)
    }

    private fun applyKotlin(owner: KtModifierListOwner, ruleId: String) {
        val factory = KtPsiFactory(owner.project)
        val existing = owner.annotationEntries.firstOrNull {
            it.shortName?.asString() == "Suppress"
        }
        if (existing == null) {
            owner.addAnnotationEntry(factory.createAnnotationEntry("@Suppress(\"$ruleId\")"))
            return
        }
        val merged = mergeArguments(extractKotlinSuppressArgs(existing), ruleId) ?: return
        val replacement = factory.createAnnotationEntry(
            "@Suppress(${merged.joinToString(", ") { "\"$it\"" }})",
        )
        existing.replace(replacement)
    }

    private fun applyJava(owner: PsiModifierListOwner, ruleId: String) {
        val modifierList = owner.modifierList ?: return
        val existing = modifierList.findAnnotation("java.lang.SuppressWarnings")
            ?: modifierList.findAnnotation("SuppressWarnings")
        val current = if (existing != null) extractJavaSuppressArgs(existing) else emptyList()
        val merged = mergeArguments(current, ruleId) ?: return
        val factory = JavaPsiFacade.getElementFactory(owner.project)
        val annotation = factory.createAnnotationFromText(
            "@SuppressWarnings(${formatJavaSuppressValue(merged)})",
            owner,
        )
        if (existing != null) {
            existing.replace(annotation)
        } else {
            modifierList.addBefore(annotation, modifierList.firstChild)
        }
    }

    internal fun extractKotlinSuppressArgs(entry: KtAnnotationEntry): List<String> {
        return entry.valueArguments.mapNotNull { arg ->
            val text = arg.getArgumentExpression()?.text ?: return@mapNotNull null
            stripQuotes(text)
        }
    }

    internal fun extractJavaSuppressArgs(annotation: PsiAnnotation): List<String> {
        val value = annotation.findAttributeValue("value") ?: return emptyList()
        return when (value) {
            is PsiLiteralExpression -> listOfNotNull(value.value as? String)
            is PsiArrayInitializerMemberValue -> value.initializers.mapNotNull {
                (it as? PsiLiteralExpression)?.value as? String
            }
            else -> emptyList()
        }
    }

    internal fun stripQuotes(text: String): String {
        val trimmed = text.trim()
        return if (trimmed.length >= 2 && trimmed.first() == '"' && trimmed.last() == '"') {
            trimmed.substring(1, trimmed.length - 1)
        } else {
            trimmed
        }
    }
}

class KritSuppressIntention(private val ruleId: String) : IntentionAction {
    override fun getText(): String = KritSuppress.titleFor(ruleId)

    override fun getFamilyName(): String = KritSuppress.SUPPRESS_FAMILY

    override fun isAvailable(project: Project, editor: Editor?, file: PsiFile?): Boolean {
        if (file == null || editor == null) return false
        val element = file.findElementAt(editor.caretModel.offset) ?: return false
        return KritSuppress.enclosingOwner(element) != null
    }

    override fun invoke(project: Project, editor: Editor?, file: PsiFile?) {
        val ed = editor ?: return
        val f = file ?: return
        val element = f.findElementAt(ed.caretModel.offset) ?: return
        WriteCommandAction.runWriteCommandAction(project) {
            KritSuppress.applyToElement(element, ruleId)
        }
    }

    override fun startInWriteAction(): Boolean = false
}

class KritSuppressQuickFix(private val ruleId: String) : LocalQuickFix {
    override fun getFamilyName(): String = KritSuppress.titleFor(ruleId)

    override fun applyFix(project: Project, descriptor: ProblemDescriptor) {
        val element = descriptor.psiElement ?: return
        KritSuppress.applyToElement(element, ruleId)
    }
}
