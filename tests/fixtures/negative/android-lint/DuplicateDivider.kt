package com.example.ui

import androidx.recyclerview.widget.RecyclerView

// Lint only matches the support-library ItemDecoration, and only the sample's two orientation fields.
class DividerItemDecoration : RecyclerView.ItemDecoration() {
    companion object {
        const val HORIZONTAL_LIST = 0
        const val VERTICAL_LIST = 1
    }
}

class SpacingDecoration : android.support.v7.widget.RecyclerView.ItemDecoration() {
    val HORIZONTAL_LIST = 0
    val VERTICAL_LIST = 1
}
