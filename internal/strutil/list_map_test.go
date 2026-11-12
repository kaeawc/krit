package strutil

import (
	"reflect"
	"testing"
)

func TestDecodeListMap(t *testing.T) {
	got := DecodeListMap("af=NA,ZA bm= ar=a,b,c\n  x=y")
	want := map[string][]string{"af": {"NA", "ZA"}, "bm": {}, "ar": {"a", "b", "c"}, "x": {"y"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got["bm"] == nil {
		t.Fatal("empty list must be non-nil so presence checks keep working")
	}
}
