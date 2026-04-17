package com.example

import androidx.fragment.app.FragmentManager

class FragScreen {
    fun showFragment(manager: FragmentManager) {
        val tx = manager.beginTransaction()
        tx.replace(android.R.id.content, MyFragment())
        // Missing commit()
    }
}
