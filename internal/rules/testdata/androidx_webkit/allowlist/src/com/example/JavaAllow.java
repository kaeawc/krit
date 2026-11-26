package com.example;

import androidx.webkit.Profile;
import java.util.Collections;

class JavaAllow {
    void allow(Profile profile) {
        profile.setCrossOriginIsolatedAllowlist(Collections.singleton("https://example.com"));
    }
}
