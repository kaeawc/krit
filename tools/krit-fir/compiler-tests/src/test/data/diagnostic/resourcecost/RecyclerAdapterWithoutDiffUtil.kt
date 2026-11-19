// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 31, 49, 67, 87, 102, 113
// Adapters that call notifyDataSetChanged() with no DiffUtil or ListAdapter in the class.
package test

import android.annotation.SuppressLint
import android.view.View
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView

class Holder(view: View) : RecyclerView.ViewHolder(view)

class PlainAdapter : RecyclerView.Adapter<Holder>() {
    private var items = listOf<String>()

    fun update(newItems: List<String>) {
        items = newItems
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = items.size
}

// Only the first call is reported.
class TwoCallsAdapter : RecyclerView.Adapter<Holder>() {
    fun first() {
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    fun second() {
        notifyDataSetChanged()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// A call inside a lambda, with an explicit receiver.
class LambdaAdapter : RecyclerView.Adapter<Holder>() {
    fun load(source: List<String>) {
        source.forEach { println(it) }
        run { this.<!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>() }
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// The first call is suppressed; the next unsuppressed call is the one reported.
class SecondUnsuppressedAdapter : RecyclerView.Adapter<Holder>() {
    @SuppressLint("NotifyDataSetChanged")
    fun first() {
        notifyDataSetChanged()
    }

    fun second() {
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}

// A nested adapter's call is reported once, for the nested adapter (RecyclerAdapterWithoutDiffUtilNestedTest counts it).
class OuterAdapter : RecyclerView.Adapter<Holder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0

    class InnerAdapter : RecyclerView.Adapter<Holder>() {
        fun refresh() {
            <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
        }

        override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

        override fun onBindViewHolder(holder: Holder, position: Int) = Unit

        override fun getItemCount(): Int = 0
    }
}

// The enclosing adapter's own call, after a nested adapter's call, is its finding.
class HostOfNested : RecyclerView.Adapter<Holder>() {
    class Nested : RecyclerView.Adapter<Holder>() {
        fun refresh() {
            <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
        }

        override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

        override fun onBindViewHolder(holder: Holder, position: Int) = Unit

        override fun getItemCount(): Int = 0
    }

    fun reload() {
        <!RecyclerAdapterWithoutDiffUtil!>notifyDataSetChanged<!>()
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): Holder = TODO()

    override fun onBindViewHolder(holder: Holder, position: Int) = Unit

    override fun getItemCount(): Int = 0
}
