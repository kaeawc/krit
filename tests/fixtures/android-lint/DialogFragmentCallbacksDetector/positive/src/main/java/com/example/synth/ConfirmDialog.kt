package com.example.synth

import android.app.Dialog
import android.os.Bundle
import androidx.appcompat.app.AlertDialog
import androidx.fragment.app.DialogFragment

class ConfirmDialog : DialogFragment() {
    override fun onCreateDialog(savedInstanceState: Bundle?): Dialog {
        val dialog = AlertDialog.Builder(requireContext()).setMessage("Delete?").create()
        dialog.setOnCancelListener { dismissAllowingStateLoss() }
        dialog.setOnDismissListener { }
        return dialog
    }
}
