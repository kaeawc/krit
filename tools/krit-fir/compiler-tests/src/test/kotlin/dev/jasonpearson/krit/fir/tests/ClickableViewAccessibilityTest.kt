package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

class ClickableViewAccessibilityTest {
    @Test
    fun messagesAndNameAnchorsAlsoApplyToTestFiles() {
        val source = """
            package clickable.messages
            import android.content.Context
            import android.view.MotionEvent
            import android.view.View
            class Absent(context: Context) : View(context) {
                override fun onTouchEvent(event: MotionEvent): Boolean = true
            }
            class Present(context: Context) : View(context) {
                override fun
                    onTouchEvent(event: MotionEvent): Boolean = true
                override fun performClick(): Boolean = false
            }
            fun attach(view: View) {
                view.setOnTouchListener { _, _ -> true }
                view.setOnTouchListener(object : View.OnTouchListener {
                    override fun onTouch(view: View, event: MotionEvent): Boolean = false
                })
            }
        """.trimIndent()
        val result = KritFirProbe.compile(
            mapOf("TouchTest.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("ClickableViewAccessibility")),
            testFiles = setOf("TouchTest.kt"),
        )
        check(result.clean) { result.problems() }
        val prefix = "[ClickableViewAccessibility] "
        assertEquals(
            mapOf(
                6 to "Custom view `Absent` overrides `onTouchEvent` but not `performClick`",
                10 to "`Present#onTouchEvent` should call `Present#performClick` when a click is detected",
                11 to "`Present#performClick` should call `super#performClick`",
                14 to "`onTouch` lambda should call `View#performClick` when a click is detected",
                16 to "`onTouch` should call `View#performClick` when a click is detected",
            ),
            result.diags.associate { it.line to it.message.substringAfter(prefix) },
        )
    }
}
