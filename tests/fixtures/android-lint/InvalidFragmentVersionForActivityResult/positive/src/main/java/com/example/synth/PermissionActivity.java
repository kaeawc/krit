package com.example.synth;

import androidx.activity.ComponentActivity;
import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;

public class PermissionActivity extends ComponentActivity {
    private final ActivityResultLauncher<String> request =
            registerForActivityResult(new ActivityResultContracts.RequestPermission(), granted -> { });
}
