package arch

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestSymbolFanIn_Empty(t *testing.T) {
	idx := scanner.BuildIndexFromData(nil, nil)
	fanIn := SymbolFanIn(idx)
	if len(fanIn) != 0 {
		t.Errorf("expected empty map, got %v", fanIn)
	}
}

func TestSymbolFanIn_SingleRef(t *testing.T) {
	symbols := []scanner.Symbol{
		{Name: "Foo", Kind: "class", File: "a.kt", Line: 1},
	}
	refs := []scanner.Reference{
		{Name: "Foo", File: "a.kt", Line: 5},
		{Name: "Foo", File: "b.kt", Line: 3},
	}
	idx := scanner.BuildIndexFromData(symbols, refs)
	fanIn := SymbolFanIn(idx)

	if fanIn["Foo"] != 1 {
		t.Errorf("expected fan-in 1 for Foo (one external file), got %d", fanIn["Foo"])
	}
}

func TestSymbolFanIn_HighFanIn(t *testing.T) {
	symbols := []scanner.Symbol{
		{Name: "Utils", Kind: "object", File: "utils.kt", Line: 1},
	}
	refs := []scanner.Reference{
		{Name: "Utils", File: "utils.kt", Line: 10},
		{Name: "Utils", File: "a.kt", Line: 2},
		{Name: "Utils", File: "b.kt", Line: 3},
		{Name: "Utils", File: "c.kt", Line: 4},
		{Name: "Utils", File: "d.kt", Line: 5},
		{Name: "Utils", File: "e.kt", Line: 6},
	}
	idx := scanner.BuildIndexFromData(symbols, refs)
	fanIn := SymbolFanIn(idx)

	if fanIn["Utils"] != 5 {
		t.Errorf("expected fan-in 5 for Utils, got %d", fanIn["Utils"])
	}
}

func TestFilterHotspots_Threshold(t *testing.T) {
	symbols := []scanner.Symbol{
		{Name: "Hot", Kind: "class", File: "hot.kt", Line: 1},
		{Name: "Cold", Kind: "class", File: "cold.kt", Line: 1},
	}
	refs := []scanner.Reference{
		{Name: "Hot", File: "a.kt", Line: 1},
		{Name: "Hot", File: "b.kt", Line: 1},
		{Name: "Hot", File: "c.kt", Line: 1},
		{Name: "Hot", File: "d.kt", Line: 1},
		{Name: "Cold", File: "x.kt", Line: 1},
	}
	idx := scanner.BuildIndexFromData(symbols, refs)
	fanIn := SymbolFanIn(idx)

	hotspots := FilterHotspots(idx, fanIn, 3)
	if len(hotspots) != 1 {
		t.Fatalf("expected 1 hotspot above threshold 3, got %d", len(hotspots))
	}
	if hotspots[0].Name != "Hot" {
		t.Errorf("expected Hot, got %s", hotspots[0].Name)
	}
	if hotspots[0].FanIn != 4 {
		t.Errorf("expected fan-in 4, got %d", hotspots[0].FanIn)
	}
}

func TestFilterHotspots_SortOrder(t *testing.T) {
	symbols := []scanner.Symbol{
		{Name: "Low", Kind: "class", File: "low.kt", Line: 1},
		{Name: "Mid", Kind: "function", File: "mid.kt", Line: 1},
		{Name: "High", Kind: "object", File: "high.kt", Line: 1},
	}
	refs := []scanner.Reference{
		// Low: 2 external files
		{Name: "Low", File: "a.kt", Line: 1},
		{Name: "Low", File: "b.kt", Line: 1},
		// Mid: 3 external files
		{Name: "Mid", File: "a.kt", Line: 1},
		{Name: "Mid", File: "b.kt", Line: 1},
		{Name: "Mid", File: "c.kt", Line: 1},
		// High: 5 external files
		{Name: "High", File: "a.kt", Line: 1},
		{Name: "High", File: "b.kt", Line: 1},
		{Name: "High", File: "c.kt", Line: 1},
		{Name: "High", File: "d.kt", Line: 1},
		{Name: "High", File: "e.kt", Line: 1},
	}
	idx := scanner.BuildIndexFromData(symbols, refs)
	fanIn := SymbolFanIn(idx)

	hotspots := FilterHotspots(idx, fanIn, 0)
	if len(hotspots) != 3 {
		t.Fatalf("expected 3 hotspots, got %d", len(hotspots))
	}
	if hotspots[0].Name != "High" {
		t.Errorf("expected High first, got %s", hotspots[0].Name)
	}
	if hotspots[1].Name != "Mid" {
		t.Errorf("expected Mid second, got %s", hotspots[1].Name)
	}
	if hotspots[2].Name != "Low" {
		t.Errorf("expected Low third, got %s", hotspots[2].Name)
	}
}
