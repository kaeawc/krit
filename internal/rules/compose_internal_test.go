package rules

import "testing"

func TestComposeModifierTextHasPreviewAnnotation(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"simple Preview", "@Preview", true},
		{"qualified Preview", "@androidx.compose.ui.tooling.preview.Preview", true},
		{"Preview with args", "@Preview(showBackground = true)", true},
		{"trailing Preview suffix", "@MultiDevicePreview", true},
		{"PreviewParameter does not match", "@PreviewParameter", false},
		{"Previewish does not match", "@Previewish", false},
		{"MyPreviewState does not match", "@MyPreviewState", false},
		{"unrelated annotation", "@Composable", false},
		{"no annotation", "public", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := composeModifierTextHasPreviewAnnotation(tc.text); got != tc.want {
				t.Fatalf("composeModifierTextHasPreviewAnnotation(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
