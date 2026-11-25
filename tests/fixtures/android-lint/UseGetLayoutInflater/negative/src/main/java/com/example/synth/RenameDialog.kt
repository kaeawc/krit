package com.example.synth

import android.app.Dialog
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import androidx.appcompat.app.AlertDialog
import androidx.fragment.app.DialogFragment
import androidx.fragment.app.Fragment

class RenameDialog : DialogFragment() {
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog {
        val content = layoutInflater.inflate(android.R.layout.simple_list_item_1, null)
        return AlertDialog.Builder(requireContext()).setView(content).create()
    }
}

// A plain Fragment inflates with the context's theme.
class ListFragment : Fragment() {
    fun row(): View = LayoutInflater.from(requireContext()).inflate(android.R.layout.simple_list_item_1, null)
}
