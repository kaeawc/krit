package com.example.synth

import android.os.Bundle
import android.view.Menu
import android.view.MenuInflater
import android.view.MenuItem
import android.view.View
import androidx.core.view.MenuProvider
import androidx.fragment.app.Fragment
import androidx.lifecycle.Lifecycle

class ItemsFragment : Fragment() {
    private val menu = object : MenuProvider {
        override fun onCreateMenu(menu: Menu, menuInflater: MenuInflater) {}
        override fun onMenuItemSelected(menuItem: MenuItem) = false
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        requireActivity().addMenuProvider(menu, this)
        requireActivity().addMenuProvider(menu, this, Lifecycle.State.RESUMED)
    }
}
