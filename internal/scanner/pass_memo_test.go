package scanner

import (
	"sync"
	"testing"
)

func TestPassMemo(t *testing.T) {
	var nilMemo *PassMemo
	nilMemo.Store(1, 7, "x")
	if _, ok := nilMemo.Load(1, 7); ok {
		t.Fatal("a nil memo stored a value")
	}
	m := NewPassMemo()
	m.Store(1, 7, "a")
	m.Store(2, 7, "b")
	if v, ok := m.Load(1, 7); !ok || v != "a" {
		t.Errorf("Load(1, 7) = %v, %v", v, ok)
	}
	if v, ok := m.Load(2, 7); !ok || v != "b" {
		t.Errorf("Load(2, 7) = %v, %v: slots must not collide", v, ok)
	}
	if _, ok := m.Load(1, 8); ok {
		t.Error("Load(1, 8) found a value that was never stored")
	}
	// Project-scope rules may read a file while its dispatch pass writes the memo.
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range uint32(1000) {
				m.Store(3, i, g)
				m.Load(3, i)
			}
		})
	}
	wg.Wait()
	f := &File{}
	if f.PassMemo() != nil {
		t.Fatal("a new file has a pass memo")
	}
	f.SetPassMemo(m)
	if f.PassMemo() != m {
		t.Fatal("PassMemo did not return the published memo")
	}
	f.SetPassMemo(nil)
	if f.PassMemo() != nil {
		t.Fatal("SetPassMemo(nil) did not clear the memo")
	}
}
