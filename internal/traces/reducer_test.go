package traces

import "testing"

func TestReduceCollapsesAdjacentEqualEvents(t *testing.T) {
	events := []Event{
		{TimestampNS: 1, FrameStack: []string{"com.acme.Foo.bar"}, Kind: KindCall, Role: RoleRequest, SourceID: "s1"},
		{TimestampNS: 2, FrameStack: []string{"com.acme.Foo.bar"}, Kind: KindCall, Role: RoleRequest, SourceID: "s1"},
		{TimestampNS: 3, FrameStack: []string{"com.acme.Foo.bar"}, Kind: KindCall, Role: RoleRequest, SourceID: "s1"},
	}
	states, trans := Reduce(events)
	if len(states) != 1 {
		t.Fatalf("want 1 state, got %d", len(states))
	}
	if states[0].Count != 3 {
		t.Errorf("want count 3, got %d", states[0].Count)
	}
	if len(trans) != 0 {
		t.Errorf("want 0 transitions, got %d", len(trans))
	}
	if states[0].FirstSeen != 1 || states[0].LastSeen != 3 {
		t.Errorf("timestamps: got first=%d last=%d", states[0].FirstSeen, states[0].LastSeen)
	}
}

func TestReduceEmitsTransitionOnStateChange(t *testing.T) {
	events := []Event{
		{TimestampNS: 1, FrameStack: []string{"a"}, Role: RoleRequest, SourceID: "s1"},
		{TimestampNS: 2, FrameStack: []string{"b"}, Role: RoleRequest, SourceID: "s1"},
		{TimestampNS: 3, FrameStack: []string{"a"}, Role: RoleRequest, SourceID: "s1"},
	}
	states, trans := Reduce(events)
	if len(states) != 2 {
		t.Fatalf("want 2 states, got %d", len(states))
	}
	if len(trans) != 2 {
		t.Fatalf("want 2 transitions, got %d", len(trans))
	}
}

func TestReduceRoleSeparatesStates(t *testing.T) {
	events := []Event{
		{FrameStack: []string{"com.acme.Foo.bar"}, Role: RoleStartup, SourceID: "s1"},
		{FrameStack: []string{"com.acme.Foo.bar"}, Role: RoleTest, SourceID: "s2"},
	}
	states, _ := Reduce(events)
	if len(states) != 2 {
		t.Fatalf("role separation: want 2 states, got %d", len(states))
	}
}

func TestFingerprintStableAcrossCallerWindow(t *testing.T) {
	a := Fingerprint("Foo.bar", HashCallerChain([]string{"Foo.bar", "X", "Y", "Z"}), RoleRequest)
	b := Fingerprint("Foo.bar", HashCallerChain([]string{"Foo.bar", "X", "Y", "Z"}), RoleRequest)
	if a != b {
		t.Fatalf("fingerprint not stable: %s vs %s", a, b)
	}
	c := Fingerprint("Foo.bar", HashCallerChain([]string{"Foo.bar", "X", "Y", "W"}), RoleRequest)
	if a == c {
		t.Fatalf("caller-chain change must change fingerprint")
	}
}

func TestReduceSkipsEmptyStack(t *testing.T) {
	states, trans := Reduce([]Event{{FrameStack: nil, SourceID: "s1"}})
	if len(states) != 0 || len(trans) != 0 {
		t.Fatalf("empty stack must not produce a state")
	}
}
