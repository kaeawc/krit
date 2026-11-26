package com.example;

import slack.lint.annotations.JavaOnly;

public class LegacyApi {
    @JavaOnly
    public static String load() {
        return "legacy";
    }

    public static String loadAsync() {
        return "async";
    }
}
