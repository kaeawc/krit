package fixtures.positive.resourcecost

import androidx.recyclerview.widget.RecyclerView

class ImageViewHolder(view: android.view.View) : RecyclerView.ViewHolder(view) {
    fun bind(url: String) {
        Glide.with(itemView.context)
            .load(url)
            .into(imageView)
    }
}
