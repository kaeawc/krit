package module

import "testing"

func TestAccessorToPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"circuitRuntime", ":circuit-runtime"},
		{"sentryAndroidCore", ":sentry-android-core"},
		{"backstack", ":backstack"},
		{"circuitCodegenAnnotations", ":circuit-codegen-annotations"},
		{"internalTestUtils", ":internal-test-utils"},
		{"circuitRuntimeUi", ":circuit-runtime-ui"},
		{"app", ":app"},
		{"", ":"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := AccessorToPath(tt.input)
			if got != tt.want {
				t.Errorf("AccessorToPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
