package faction

import "testing"

func TestAllFactionsDefined(t *testing.T) {
	all := All()
	if len(all) != int(Count) {
		t.Fatalf("got %d factions, want %d", len(all), Count)
	}
	seen := map[string]bool{}
	for i, d := range all {
		if d.ID != ID(i) {
			t.Errorf("faction %d has ID %d", i, d.ID)
		}
		if d.Name == "" || d.Shift == "" {
			t.Errorf("faction %d missing name or shift", i)
		}
		if seen[d.Name] {
			t.Errorf("duplicate faction name %q", d.Name)
		}
		seen[d.Name] = true
	}
}

func TestNextWraps(t *testing.T) {
	if got := Forge.Next(); got != Brood {
		t.Errorf("Forge.Next() = %d, want Brood", got)
	}
	if got := Phasebound.Next(); got != Forge {
		t.Errorf("Phasebound.Next() = %d, want Forge", got)
	}
}
