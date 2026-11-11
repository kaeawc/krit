package rules

import (
	"reflect"
	"testing"
)

func TestDecodeListMap(t *testing.T) {
	got := decodeListMap("af=NA,ZA bm= ar=a,b,c")
	want := map[string][]string{"af": {"NA", "ZA"}, "bm": {}, "ar": {"a", "b", "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got["bm"] == nil {
		t.Fatal("empty list must be non-nil so presence checks keep working")
	}
}
