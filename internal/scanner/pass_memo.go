package scanner

import "sync"

// PassMemo holds facts that rules derive from one file's nodes, shared by the rules of one dispatch pass over the
// file. The rule dispatcher publishes a fresh memo with File.SetPassMemo while it runs the file and clears it after,
// so the facts do not outlive the pass. Other goroutines may read the file during the pass (project-scope rules
// run beside per-file dispatch), so access is locked.
type PassMemo struct {
	mu   sync.Mutex
	vals map[passMemoKey]any
}

// PassMemoSlot distinguishes the kinds of fact stored on the same node.
type PassMemoSlot uint16

type passMemoKey struct {
	slot PassMemoSlot
	idx  uint32
}

// NewPassMemo returns an empty memo.
func NewPassMemo() *PassMemo { return &PassMemo{vals: map[passMemoKey]any{}} }

// Load returns the fact stored for the node under slot. A nil memo stores nothing.
func (m *PassMemo) Load(slot PassMemoSlot, idx uint32) (any, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.Lock()
	v, ok := m.vals[passMemoKey{slot, idx}]
	m.mu.Unlock()
	return v, ok
}

// Store records the fact for the node under slot. A nil memo ignores it.
func (m *PassMemo) Store(slot PassMemoSlot, idx uint32, v any) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.vals[passMemoKey{slot, idx}] = v
	m.mu.Unlock()
}

// PassMemo returns the memo published for f's current dispatch pass, or nil.
func (f *File) PassMemo() *PassMemo {
	if f == nil {
		return nil
	}
	return f.passMemo.Load()
}

// SetPassMemo publishes m for f (nil clears it).
func (f *File) SetPassMemo(m *PassMemo) {
	f.passMemo.Store(m)
}
