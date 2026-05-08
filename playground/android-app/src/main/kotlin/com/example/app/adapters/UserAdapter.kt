package com.example.app.adapters

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.ImageView
import android.widget.TextView
import androidx.recyclerview.widget.RecyclerView
import com.example.app.R

// ViewHolder pattern triggers
class UserAdapter(
    private var users: List<UserItem>
) : RecyclerView.Adapter<UserAdapter.ViewHolder>() {

    class ViewHolder(itemView: View) : RecyclerView.ViewHolder(itemView) {
        val nameText: TextView = itemView.findViewById(R.id.text_name)
        val emailText: TextView = itemView.findViewById(R.id.text_email)
        val avatar: ImageView = itemView.findViewById(R.id.img_avatar)
    }

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): ViewHolder {
        val view = LayoutInflater.from(parent.context)
            .inflate(R.layout.item_user, parent, false)
        return ViewHolder(view)
    }

    override fun onBindViewHolder(holder: ViewHolder, position: Int) {
        val user = users[position]
        holder.nameText.text = user.name
        holder.emailText.text = user.email
        // MagicNumber: hardcoded alpha
        holder.avatar.alpha = 0.8f
    }

    override fun getItemCount(): Int = users.size

    // Inefficient notifyDataSetChanged instead of DiffUtil
    fun updateUsers(newUsers: List<UserItem>) {
        users = newUsers
        notifyDataSetChanged()
    }

    // SwallowedException
    fun getUser(position: Int): UserItem? {
        try {
            return users[position]
        } catch (e: IndexOutOfBoundsException) {
            return null
        }
    }
}

data class UserItem(
    val id: String,
    val name: String,
    val email: String,
    val avatarUrl: String?
)
