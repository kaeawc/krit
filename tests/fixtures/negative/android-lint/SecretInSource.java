package com.example;

public class Assistant {
    static class GenerativeModel {
        GenerativeModel(String name, String apiKey) {}
    }

    GenerativeModel create() {
        return new GenerativeModel("gemini-pro", "AIzaSyH0000000000000000000000000000000");
    }
}
