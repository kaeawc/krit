package com.example.synth;

import android.os.Bundle;
import android.widget.Toolbar;
import androidx.appcompat.app.AppCompatActivity;

public class SettingsActivity extends AppCompatActivity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setActionBar(new Toolbar(this));
    }
}
