package com.example;

import com.google.ai.client.generativeai.GenerativeModel;

public class Assistant {
    private static final String KEY = "AIzaSyD0000000000000000000000000000000";

    GenerativeModel create() {
        return new GenerativeModel("gemini-pro", KEY);
    }
}
