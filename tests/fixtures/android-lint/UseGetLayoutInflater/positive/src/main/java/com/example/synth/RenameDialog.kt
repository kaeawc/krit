package com.example.synth

import android.app.Dialog
import android.os.Bundle
import android.view.LayoutInflater
import androidx.appcompat.app.AlertDialog
import androidx.fragment.app.DialogFragment

class RenameDialog : DialogFragment() {
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog {
        val content = LayoutInflater.from(requireContext()).inflate(android.R.layout.simple_list_item_1, null)
        return AlertDialog.Builder(requireContext()).setView(content).create()
    }
}
