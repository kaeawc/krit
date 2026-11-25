package com.example.synth;

import android.content.Context;
import androidx.fragment.app.Fragment;
import java.util.Objects;

public class JavaProfileFragment extends Fragment {
    Context context() {
        return Objects.requireNonNull(getContext());
    }
}
