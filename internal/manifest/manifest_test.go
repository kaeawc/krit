package manifest

import "testing"

func TestAllComponents_Nil(t *testing.T) {
	if got := AllComponents(nil); got != nil {
		t.Fatalf("AllComponents(nil) = %v, want nil", got)
	}
}

func TestAllComponents_Empty(t *testing.T) {
	got := AllComponents(&Application{})
	if len(got) != 0 {
		t.Fatalf("AllComponents(&Application{}) len = %d, want 0", len(got))
	}
}

func TestAllComponents_OrderActivitiesServicesReceiversProviders(t *testing.T) {
	app := &Application{
		Activities: []Component{{Tag: "activity", Name: "A1"}, {Tag: "activity", Name: "A2"}},
		Services:   []Component{{Tag: "service", Name: "S1"}},
		Receivers:  []Component{{Tag: "receiver", Name: "R1"}, {Tag: "receiver", Name: "R2"}},
		Providers:  []Component{{Tag: "provider", Name: "P1"}},
	}
	got := AllComponents(app)

	wantTags := []string{"activity", "activity", "service", "receiver", "receiver", "provider"}
	if len(got) != len(wantTags) {
		t.Fatalf("AllComponents len = %d, want %d", len(got), len(wantTags))
	}
	for i, c := range got {
		if c.Tag != wantTags[i] {
			t.Errorf("index %d: tag = %q, want %q", i, c.Tag, wantTags[i])
		}
	}

	wantNames := []string{"A1", "A2", "S1", "R1", "R2", "P1"}
	for i, c := range got {
		if c.Name != wantNames[i] {
			t.Errorf("index %d: name = %q, want %q", i, c.Name, wantNames[i])
		}
	}
}

func TestAllComponents_DoesNotAliasInputSlices(t *testing.T) {
	app := &Application{
		Activities: []Component{{Tag: "activity", Name: "A1"}},
		Services:   []Component{{Tag: "service", Name: "S1"}},
	}
	got := AllComponents(app)

	got[0].Name = "mutated"
	if app.Activities[0].Name != "A1" {
		t.Fatalf("mutating result aliased back into Activities: got %q", app.Activities[0].Name)
	}
}

func TestManifestZeroValueIsUsable(t *testing.T) {
	var m Manifest
	if m.Package != "" || m.MinSDK != 0 || m.Application != nil {
		t.Fatalf("zero-value Manifest unexpectedly populated: %+v", m)
	}
	if got := AllComponents(m.Application); got != nil {
		t.Fatalf("AllComponents on zero-value app pointer = %v, want nil", got)
	}
}
