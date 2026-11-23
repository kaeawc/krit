package scanner

import "slices"

// CallNameMemo holds the callee name of every call_expression in one file,
// computed once so the many rules that ask for the same call's name share the
// work. Nodes is the file's call_expression posting list (ascending flat
// indices) and Names[i] is the name for Nodes[i]. A memo is read-only once
// published with File.SetCallNameMemo.
type CallNameMemo struct {
	Nodes []uint32
	Names []string
}

// Lookup returns the memoized name of the call_expression at idx. ok is false
// when idx is not one of the memoized nodes; callers then compute the name.
func (m *CallNameMemo) Lookup(idx uint32) (name string, ok bool) {
	if m == nil {
		return "", false
	}
	i, found := slices.BinarySearch(m.Nodes, idx)
	if !found {
		return "", false
	}
	return m.Names[i], true
}

// CallNameMemo returns the call-name memo currently published for f, or nil.
func (f *File) CallNameMemo() *CallNameMemo {
	if f == nil {
		return nil
	}
	return f.callNames.Load()
}

// SetCallNameMemo publishes m for f (nil clears it). The rule dispatcher
// publishes a memo for the duration of one file's dispatch pass; concurrent
// readers either see the whole memo or none and fall back to computing names.
func (f *File) SetCallNameMemo(m *CallNameMemo) {
	f.callNames.Store(m)
}
