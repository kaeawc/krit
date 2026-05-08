package test;

class AwsConfig {
    // Loaded at runtime — must not be flagged.
    static String loadKey() {
        return System.getenv("AWS_ACCESS_KEY_ID");
    }

    // AWS docs example marker — must not be flagged.
    static final String DOCS_EXAMPLE = "AKIAIOSFODNN7EXAMPLE";
}
