// Compiler-test source stubs; never packaged in the production artifact.
package androidx.recyclerview.widget

import android.content.Context
import android.util.AttributeSet
import android.view.View
import android.view.ViewGroup

open class RecyclerView : ViewGroup {
    constructor(context: Context) : super(context)

    constructor(context: Context, attrs: AttributeSet?) : super(context, attrs)

    constructor(context: Context, attrs: AttributeSet?, defStyleAttr: Int) : super(context, attrs, defStyleAttr)

    // Java getAdapter()/setAdapter(Adapter) with a raw type.
    open var adapter: Adapter<*>?
        get() = TODO()
        set(value) = TODO()

    open var layoutManager: LayoutManager?
        get() = TODO()
        set(value) = TODO()

    open fun setHasFixedSize(hasFixedSize: Boolean) {
        TODO()
    }

    open fun addItemDecoration(decor: ItemDecoration) {
        TODO()
    }

    open fun addOnScrollListener(listener: OnScrollListener) {
        TODO()
    }

    open fun scrollToPosition(position: Int) {
        TODO()
    }

    open fun smoothScrollToPosition(position: Int) {
        TODO()
    }

    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        TODO()
    }

    abstract class ViewHolder(@JvmField val itemView: View) {
        @Deprecated("Deprecated in Java")
        val adapterPosition: Int
            get() = TODO()

        val bindingAdapterPosition: Int
            get() = TODO()

        val absoluteAdapterPosition: Int
            get() = TODO()

        val layoutPosition: Int
            get() = TODO()

        val itemViewType: Int
            get() = TODO()
    }

    abstract class Adapter<VH : ViewHolder> {
        abstract fun onCreateViewHolder(parent: ViewGroup, viewType: Int): VH

        abstract fun onBindViewHolder(holder: VH, position: Int)

        open fun onBindViewHolder(holder: VH, position: Int, payloads: MutableList<Any>) {
            TODO()
        }

        // Overridden by app code, so it stays a function (call sites use
        // getItemCount(), not the Java-synthetic `itemCount`).
        abstract fun getItemCount(): Int

        open fun getItemViewType(position: Int): Int = TODO()

        open fun getItemId(position: Int): Long = TODO()

        fun setHasStableIds(hasStableIds: Boolean) {
            TODO()
        }

        open fun onViewRecycled(holder: VH) {
            TODO()
        }

        open fun onAttachedToRecyclerView(recyclerView: RecyclerView) {
            TODO()
        }

        fun notifyDataSetChanged() {
            TODO()
        }

        fun notifyItemChanged(position: Int) {
            TODO()
        }

        fun notifyItemInserted(position: Int) {
            TODO()
        }

        fun notifyItemRemoved(position: Int) {
            TODO()
        }

        fun notifyItemRangeChanged(positionStart: Int, itemCount: Int) {
            TODO()
        }
    }

    abstract class LayoutManager

    abstract class ItemDecoration

    abstract class OnScrollListener {
        open fun onScrollStateChanged(recyclerView: RecyclerView, newState: Int) {
            TODO()
        }

        open fun onScrolled(recyclerView: RecyclerView, dx: Int, dy: Int) {
            TODO()
        }
    }

    companion object {
        const val HORIZONTAL: Int = 0
        const val VERTICAL: Int = 1
    }
}

open class LinearLayoutManager : RecyclerView.LayoutManager {
    constructor(context: Context?)

    constructor(context: Context?, orientation: Int, reverseLayout: Boolean)

    companion object {
        const val HORIZONTAL: Int = 0
        const val VERTICAL: Int = 1
    }
}

open class GridLayoutManager : LinearLayoutManager {
    constructor(context: Context?, spanCount: Int) : super(context)

    constructor(context: Context?, spanCount: Int, orientation: Int, reverseLayout: Boolean) :
        super(context, orientation, reverseLayout)
}

abstract class ListAdapter<T, VH : RecyclerView.ViewHolder> : RecyclerView.Adapter<VH> {
    protected constructor(diffCallback: DiffUtil.ItemCallback<T>)

    open val currentList: List<T>
        get() = TODO()

    open fun submitList(list: List<T>?) {
        TODO()
    }

    open fun submitList(list: List<T>?, commitCallback: Runnable?) {
        TODO()
    }

    protected fun getItem(position: Int): T = TODO()

    override fun getItemCount(): Int = TODO()

    open fun onCurrentListChanged(previousList: List<T>, currentList: List<T>) {
        TODO()
    }
}

// Java class with a private constructor, static calculateDiff, and static
// nested classes.
object DiffUtil {
    fun calculateDiff(cb: Callback): DiffResult = TODO()

    fun calculateDiff(cb: Callback, detectMoves: Boolean): DiffResult = TODO()

    abstract class ItemCallback<T> {
        abstract fun areItemsTheSame(oldItem: T, newItem: T): Boolean

        abstract fun areContentsTheSame(oldItem: T, newItem: T): Boolean

        open fun getChangePayload(oldItem: T, newItem: T): Any? = TODO()
    }

    abstract class Callback {
        abstract fun getOldListSize(): Int

        abstract fun getNewListSize(): Int

        abstract fun areItemsTheSame(oldItemPosition: Int, newItemPosition: Int): Boolean

        abstract fun areContentsTheSame(oldItemPosition: Int, newItemPosition: Int): Boolean
    }

    class DiffResult {
        fun dispatchUpdatesTo(adapter: RecyclerView.Adapter<*>) {
            TODO()
        }
    }
}
