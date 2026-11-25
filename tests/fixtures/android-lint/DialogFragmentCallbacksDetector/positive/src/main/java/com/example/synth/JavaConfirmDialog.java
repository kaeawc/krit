package com.example.synth;

import android.app.Dialog;
import android.os.Bundle;
import androidx.appcompat.app.AlertDialog;
import androidx.fragment.app.DialogFragment;

public class JavaConfirmDialog extends DialogFragment {
    @Override
    public Dialog onCreateDialog(Bundle savedInstanceState) {
        AlertDialog dialog = new AlertDialog.Builder(requireContext()).create();
        dialog.setOnDismissListener(d -> { });
        return dialog;
    }
}
