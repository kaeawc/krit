package com.example.synth;

import androidx.activity.ComponentActivity;

public class LegacyActivity extends ComponentActivity {
    void handledOnBackPressed() {
        onBackPressed();
    }
}
