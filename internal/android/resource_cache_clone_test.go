package android

import (
	"path/filepath"
	"reflect"
	"testing"
)

// cloneResourceIndex feeds the asynchronous cache write, so a field it skips
// is silently missing from every index a later run loads (#1830: AliasItems,
// so NotSiblingResource lost an <item type="id"> declaration on warm runs).
// Fill every field, nested ones included, and require the clone to match.
func TestCloneResourceIndexCopiesEveryField(t *testing.T) {
	src := newResourceIndex()
	sv := reflect.ValueOf(src).Elem()
	for i := 0; i < sv.NumField(); i++ {
		fillNonZero(sv.Field(i), 0)
	}
	got := reflect.ValueOf(cloneResourceIndex(src)).Elem()
	for i := 0; i < sv.NumField(); i++ {
		name := sv.Type().Field(i).Name
		if name == "ResDirs" {
			// The builder sets ResDirs after merging the per-directory
			// indexes; a cached per-directory entry never carries it.
			continue
		}
		if !reflect.DeepEqual(got.Field(i).Interface(), sv.Field(i).Interface()) {
			t.Errorf("cloneResourceIndex drops or changes ResourceIndex.%s", name)
		}
	}
}

// fillNonZero sets v to a non-zero value of its type, one element deep for
// slices and maps. Recursive types (View.Children) stop at a fixed depth.
func fillNonZero(v reflect.Value, depth int) {
	if depth > 4 || !v.CanSet() {
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		fillNonZero(s.Index(0), depth+1)
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		fillNonZero(key, depth+1)
		val := reflect.New(v.Type().Elem()).Elem()
		fillNonZero(val, depth+1)
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		fillNonZero(p.Elem(), depth+1)
		v.Set(p)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fillNonZero(v.Field(i), depth+1)
		}
	}
}

// #1830 end to end: the index a cold run saved through the asynchronous
// writer and a warm run loads must equal a fresh parse of the same res/.
func TestResourceIndexFromCacheMatchesFreshParse(t *testing.T) {
	prev := ActiveResourceIndexCache()
	t.Cleanup(func() { SetActiveResourceIndexCache(prev) })
	resDir := filepath.Join("..", "..", "tests", "fixtures", "android-lint", "UnknownId", "negative", "src", "main", "res")

	SetActiveResourceIndexCache(nil)
	fresh, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.AliasItems) == 0 {
		t.Fatal("fixture lost its <item type=\"id\"> declaration; the test exercises nothing")
	}

	repo := t.TempDir()
	cold, err := NewResourceIndexCache(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ScanResourceDir(resDir); err != nil {
		t.Fatal(err)
	}
	if err := cold.Close(); err != nil { // flushes the asynchronous writes
		t.Fatal(err)
	}

	warmCache, err := NewResourceIndexCache(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = warmCache.Close() })
	warm, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	if warmCache.Stats().Hits == 0 {
		t.Fatal("the warm scan loaded nothing from the cache")
	}
	fv, wv := reflect.ValueOf(fresh).Elem(), reflect.ValueOf(warm).Elem()
	for i := 0; i < fv.NumField(); i++ {
		if !reflect.DeepEqual(wv.Field(i).Interface(), fv.Field(i).Interface()) {
			t.Errorf("cached index differs from a fresh parse in %s:\n cached %#v\n fresh  %#v",
				fv.Type().Field(i).Name, wv.Field(i).Interface(), fv.Field(i).Interface())
		}
	}
}
