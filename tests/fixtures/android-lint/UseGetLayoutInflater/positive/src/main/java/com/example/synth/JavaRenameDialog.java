package com.example.synth;

import android.app.Dialog;
import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import androidx.appcompat.app.AlertDialog;
import androidx.fragment.app.DialogFragment;

public class JavaRenameDialog extends DialogFragment {
    @Override
    public Dialog onCreateDialog(Bundle savedInstanceState) {
        View content = LayoutInflater.from(requireContext()).inflate(android.R.layout.simple_list_item_1, null);
        return new AlertDialog.Builder(requireContext()).setView(content).create();
    }
}
