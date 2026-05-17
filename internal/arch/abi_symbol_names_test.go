package arch

import (
	"reflect"
	"testing"
)

func TestAbiSignatureSimpleNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []AbiSignature
		want []string
	}{
		{
			name: "nil input returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty slice returns nil",
			in:   []AbiSignature{},
			want: nil,
		},
		{
			name: "single class signature returns its simple name",
			in: []AbiSignature{
				{Kind: "class", FQN: "p.Foo"},
			},
			want: []string{"Foo"},
		},
		{
			name: "nested member signatures include both class and method names",
			in: []AbiSignature{
				{Kind: "class", FQN: "p.Foo"},
				{Kind: "function", FQN: "p.Foo.bar"},
			},
			want: []string{"Foo", "bar"},
		},
		{
			name: "duplicate simple names from different packages dedupe",
			in: []AbiSignature{
				{Kind: "class", FQN: "a.Foo"},
				{Kind: "class", FQN: "b.Foo"},
			},
			want: []string{"Foo"},
		},
		{
			name: "empty FQN is skipped without panic",
			in: []AbiSignature{
				{Kind: "class", FQN: ""},
				{Kind: "class", FQN: "p.Foo"},
			},
			want: []string{"Foo"},
		},
		{
			name: "FQN with no dot is returned as-is (top-level no-package)",
			in: []AbiSignature{
				{Kind: "class", FQN: "TopLevel"},
			},
			want: []string{"TopLevel"},
		},
		{
			name: "FQN ending in dot is skipped (defensive)",
			in: []AbiSignature{
				{Kind: "class", FQN: "p."},
				{Kind: "class", FQN: "p.Foo"},
			},
			want: []string{"Foo"},
		},
		{
			name: "output is sorted",
			in: []AbiSignature{
				{Kind: "class", FQN: "p.zzz"},
				{Kind: "class", FQN: "p.Foo"},
				{Kind: "function", FQN: "p.Foo.bar"},
			},
			want: []string{"Foo", "bar", "zzz"},
		},
		{
			name: "only empty/dot-terminated FQNs returns nil",
			in: []AbiSignature{
				{Kind: "class", FQN: ""},
				{Kind: "class", FQN: "p."},
			},
			want: nil,
		},
		{
			name: "deeply nested member yields innermost segment",
			in: []AbiSignature{
				{Kind: "function", FQN: "org.jetbrains.kotlin.checkers.DiagnosedRange.addDiagnostic"},
			},
			want: []string{"addDiagnostic"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := AbiSignatureSimpleNames(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("AbiSignatureSimpleNames() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
