package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

// A nested adapter's call is reported once, for the nested adapter. Golden data compares (line, rule) sets, so it
// cannot see a second finding on the same line for the enclosing adapter.
class RecyclerAdapterWithoutDiffUtilNestedTest {

    private val source = """
        package nested

        import android.view.ViewGroup
        import androidx.recyclerview.widget.RecyclerView

        abstract class Rows : RecyclerView.Adapter<RecyclerView.ViewHolder>() {
            override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): RecyclerView.ViewHolder = TODO()

            override fun onBindViewHolder(holder: RecyclerView.ViewHolder, position: Int) = Unit

            override fun getItemCount(): Int = 0
        }

        class OuterAdapter : Rows() {
            class InnerAdapter : Rows() {
                fun refresh() {
                    notifyDataSetChanged()
                }
            }

            inner class Clicks {
                fun click() {
                    notifyDataSetChanged()
                }
            }
        }
    """.trimIndent()

    @Test fun eachCallBelongsToItsNearestAdapter() {
        val result = KritFirProbe.compile(
            mapOf("Nested.kt" to source),
            FirRuleCompileContext(enabledRuleIds = setOf("RecyclerAdapterWithoutDiffUtil")),
        )
        assertEquals(emptyList(), result.compileErrors, result.problems())
        val findings = result.diags
            .filter { it.name == "RecyclerAdapterWithoutDiffUtil" }
            .map { it.line to it.message.substringAfter("'").substringBefore("'") }
            .sortedBy { it.first }
        assertEquals(listOf(17 to "InnerAdapter", 23 to "OuterAdapter"), findings, result.diags.toString())
    }
}
