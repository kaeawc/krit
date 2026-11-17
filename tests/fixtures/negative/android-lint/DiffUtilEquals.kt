package com.example

import androidx.recyclerview.widget.DiffUtil

data class Row(val id: Int)

class RowCallback : DiffUtil.ItemCallback<Row>() {
    override fun areItemsTheSame(a: Row, b: Row) = a.id == b.id

    override fun areContentsTheSame(a: Row, b: Row) = a == b
}
