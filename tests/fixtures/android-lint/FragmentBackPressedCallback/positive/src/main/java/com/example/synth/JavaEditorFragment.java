package com.example.synth;

import android.os.Bundle;
import android.view.View;
import androidx.activity.OnBackPressedCallback;
import androidx.fragment.app.Fragment;

public class JavaEditorFragment extends Fragment {
    private final OnBackPressedCallback discard = new OnBackPressedCallback(true) {
        @Override
        public void handleOnBackPressed() {}
    };

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        super.onViewCreated(view, savedInstanceState);
        requireActivity().getOnBackPressedDispatcher().addCallback(this, discard);
    }
}
