package typeinfer

import (
	"reflect"
	"testing"
)

func TestDecodeListMap(t *testing.T) {
	got := decodeListMap("A= B=x C=x,y,z")
	want := map[string][]string{"A": {}, "B": {"x"}, "C": {"x", "y", "z"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got["A"] == nil {
		t.Fatal("empty value must decode to a non-nil list")
	}
}
