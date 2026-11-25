package com.example.synth

import android.app.Dialog
import android.content.DialogInterface
import android.os.Bundle
import androidx.appcompat.app.AlertDialog
import androidx.fragment.app.DialogFragment

class ConfirmDialog : DialogFragment() {
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog =
        AlertDialog.Builder(requireContext()).setMessage("Delete?").create()

    override fun onCancel(dialog: DialogInterface) {
        super.onCancel(dialog)
    }

    fun rebind(dialog: Dialog) {
        dialog.setOnDismissListener { }
    }
}
