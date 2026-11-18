package faction

import (
	"math"
	"testing"
)

func TestDamageMultipliers(t *testing.T) {
	cases := []struct {
		a    AttackType
		ar   ArmorType
		want float64
	}{
		{Normal, Light, 1}, {Normal, Armored, 1},
		{Concussive, Light, 1.25}, {Concussive, Armored, 0.6},
		{Explosive, Light, 0.6}, {Explosive, Armored, 1.5},
	}
	for _, c := range cases {
		if got := DamageMultiplier(c.a, c.ar); got != c.want {
			t.Errorf("%v vs %v = %v, want %v", c.a, c.ar, got, c.want)
		}
	}
}

func TestDamageTo(t *testing.T) {
	strider, trooper := Unit(Strider), Unit(Trooper)
	// 18 explosive vs Armored (x1.5) minus 1 armor.
	if got := strider.Weapon.DamageTo(strider); got != 26 {
		t.Errorf("Strider vs Strider = %v, want 26", got)
	}
	// 18 explosive vs Light (x0.6), no armor.
	if got := strider.Weapon.DamageTo(trooper); math.Abs(got-10.8) > 1e-9 {
		t.Errorf("Strider vs Trooper = %v, want 10.8", got)
	}
	weak := Weapon{Damage: 1, Type: Concussive}
	if got := weak.DamageTo(strider); got != MinDamage {
		t.Errorf("min damage = %v, want %v", got, MinDamage)
	}
}

func TestUnitDefsSane(t *testing.T) {
	for i, d := range Units() {
		if d.Type != UnitType(i) {
			t.Errorf("%s: Type %d at index %d", d.Name, d.Type, i)
		}
		if d.Morphs {
			// Spawn: free, immobile larva hatched by the Hive.
			if d.Crystals != 0 || d.Supply != 0 || d.Speed != 0 || len(d.Trains) == 0 {
				t.Errorf("%s: bad morpher stats %+v", d.Name, d)
			}
			continue
		}
		if d.Life > 0 {
			// Summoned: free, no supply, not trained anywhere, but a fighter.
			if d.Crystals != 0 || d.Supply != 0 || d.Speed <= 0 || !d.Armed() || d.HP <= 0 {
				t.Errorf("%s: bad summon stats %+v", d.Name, d)
			}
			if _, ok := TrainedAt(d.Type); ok {
				t.Errorf("%s: summons shouldn't be trainable", d.Name)
			}
			continue
		}
		if d.Name == "" || d.HP <= 0 || d.Radius <= 0 || d.Sight <= 0 || d.BuildTime <= 0 || d.Crystals <= 0 {
			t.Errorf("%s: bad stats %+v", d.Name, d)
		}
		if d.Building {
			if d.Size <= 0 || d.Speed != 0 || d.Supply != 0 || d.Armed() {
				t.Errorf("%s: bad building stats %+v", d.Name, d)
			}
			for _, x := range d.Trains {
				if Unit(x).Building {
					t.Errorf("%s trains building %s", d.Name, Unit(x).Name)
				}
			}
			continue
		}
		if d.Speed <= 0 || d.Supply <= 0 {
			t.Errorf("%s: bad unit stats %+v", d.Name, d)
		}
		if d.Armed() && (d.Weapon.Range <= 0 || d.Weapon.Cooldown <= 0 || d.Sight < d.Weapon.Range) {
			t.Errorf("%s: bad weapon %+v", d.Name, d.Weapon)
		}
		if !d.Armed() && d.HealRate <= 0 {
			t.Errorf("%s: neither armed nor a healer", d.Name)
		}
		if _, ok := TrainedAt(d.Type); !ok {
			t.Errorf("%s: no building trains it", d.Name)
		}
	}
}

func TestTechTree(t *testing.T) {
	got := Buildable(Forge)
	want := []UnitType{CommandHub, Generator, Extractor, DrillYard, Foundry}
	if len(got) != len(want) {
		t.Fatalf("Buildable = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Buildable = %v, want %v", got, want)
		}
	}
	if b, _ := TrainedAt(Bastion); b != Foundry {
		t.Errorf("Bastion trained at %v, want Foundry", b)
	}
	if r := Unit(Foundry).Requires; len(r) != 1 || r[0] != DrillYard {
		t.Errorf("Foundry requires %v", r)
	}
}

func TestBroodTechTree(t *testing.T) {
	got := Buildable(Brood)
	want := []UnitType{Hive, NestPod, BroodExtractor, SpawningPit, GestationDen}
	if len(got) != len(want) {
		t.Fatalf("Buildable = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Buildable = %v, want %v", got, want)
		}
	}
	for _, f := range Unit(Spawn).Trains {
		d := Unit(f)
		if d.Faction != Brood || d.Building {
			t.Errorf("Spawn morphs into %s", d.Name)
		}
		if b, _ := TrainedAt(f); b != Hive {
			t.Errorf("%s trained at %v, want Hive", d.Name, b)
		}
	}
	for _, d := range Units() {
		if d.Faction != Brood {
			continue
		}
		if d.Regen <= 0 {
			t.Errorf("%s: Brood without regen", d.Name)
		}
		if d.Building && (!d.SelfBuild || d.NeedsCreep == (d.Type == Hive || d.OnVent)) {
			t.Errorf("%s: SelfBuild %v NeedsCreep %v", d.Name, d.SelfBuild, d.NeedsCreep)
		}
	}
	if QueueSupply(Biter) != 1 || Unit(Biter).Yield() != 2 {
		t.Errorf("Biter pair supply %v yield %d", QueueSupply(Biter), Unit(Biter).Yield())
	}
	for _, f := range All() {
		if !f.Playable {
			continue
		}
		if Unit(f.HQ).Faction != f.ID || !Unit(f.Worker).Worker || Unit(f.Supply).SupplyProvided <= 0 || !Unit(f.Extractor).OnVent {
			t.Errorf("%s: bad key types", f.Name)
		}
	}
	if Forge.NextPlayable() != Brood || Brood.NextPlayable() != Forge {
		t.Error("NextPlayable should cycle Forge <-> Brood")
	}
}

func TestForgeShiftModes(t *testing.T) {
	for _, d := range Units() {
		if d.Faction != Forge {
			continue
		}
		hasMode := d.Shift != nil
		wantMode := !d.Building && !d.Worker || d.Type == CommandHub || d.Type == DrillYard || d.Type == Foundry
		if hasMode != wantMode {
			t.Errorf("%s: has SHIFT mode %v, want %v", d.Name, hasMode, wantMode)
		}
		if !hasMode && d.Shifted().Speed != d.Speed {
			t.Errorf("%s: Shifted() changed a unit without a mode", d.Name)
		}
	}
	art := Unit(Bastion).Shifted()
	if art.Speed != 0 || art.Weapon.Range != 13 || art.Weapon.Splash == 0 || art.Weapon.MinRange == 0 {
		t.Errorf("artillery: %+v speed %.1f", art.Weapon, art.Speed)
	}
	if Unit(Bastion).Weapon.Range != 7 {
		t.Error("Shifted() mutated the base def")
	}
	if p := Unit(Strider).Shifted(); p.Weapon.Range != Unit(Strider).Weapon.Range+3 || p.Speed != 0 {
		t.Errorf("planted strider: range %.0f speed %.1f", p.Weapon.Range, p.Speed)
	}
	if o := Unit(Trooper).Shifted(); o.Speed != Unit(Trooper).Speed*1.5 || o.Weapon.Cooldown >= Unit(Trooper).Weapon.Cooldown {
		t.Errorf("overdrive: speed %.2f cooldown %.2f", o.Speed, o.Weapon.Cooldown)
	}
	if b := Unit(Outrider).Shifted(); b.Armed() || b.Speed != 2*Unit(Outrider).Speed {
		t.Errorf("boost: armed %v speed %.1f", b.Armed(), b.Speed)
	}
	if m := Unit(Mender).Shifted(); !m.HealAura || m.Speed != 0 {
		t.Error("field post should be an immobile heal aura")
	}
	if h := Unit(CommandHub).Shifted(); h.Speed == 0 || h.DropOff || len(h.Trains) != 0 || h.SupplyProvided == 0 {
		t.Errorf("packed hub: speed %.1f dropoff %v trains %v supply %.0f", h.Speed, h.DropOff, h.Trains, h.SupplyProvided)
	}
}

func TestBroodShiftModes(t *testing.T) {
	for _, d := range Units() {
		if d.Faction != Brood {
			continue
		}
		hasMode := d.Shift != nil
		wantMode := !d.Building && !d.Worker && !d.Morphs && d.Life == 0
		if hasMode != wantMode {
			t.Errorf("%s: has SHIFT mode %v, want %v", d.Name, hasMode, wantMode)
		}
	}
	if b := Unit(Biter).Shifted(); !b.Burrowed || b.Speed != 0 || b.Armed() || Unit(Biter).Burrowed {
		t.Errorf("burrowed biter: hidden %v speed %.1f armed %v", b.Burrowed, b.Speed, b.Armed())
	}
	if r := Unit(Spitter).Shifted(); r.Weapon.Range != Unit(Spitter).Weapon.Range+2 || r.Speed != 0 {
		t.Errorf("rooted spitter: range %.0f speed %.1f", r.Weapon.Range, r.Speed)
	}
	if s := Unit(Behemoth).Shifted(); s.Armor != Unit(Behemoth).Armor+4 || s.Speed != Unit(Behemoth).Speed/2 || !s.Armed() {
		t.Errorf("shelled behemoth: armor %.0f speed %.1f", s.Armor, s.Speed)
	}
	for _, c := range []UnitType{Leaper, Broodmother} {
		m := Unit(c).Shift
		if !m.Cast() || m.Time != 0 || m.Duration != 0 {
			t.Errorf("%s: SHIFT should be an instant cast, got %+v", Unit(c).Name, m)
		}
		if s := Unit(c).Shifted(); s.Speed != Unit(c).Speed || s.Weapon != Unit(c).Weapon {
			t.Errorf("%s: casting shouldn't change the def", Unit(c).Name)
		}
	}
	if m := Unit(Broodmother).Shift; m.Summon != Mite || m.SummonCount != 4 || Unit(Mite).Life != 10 {
		t.Errorf("brood burst: %+v mite life %.0f", m, Unit(Mite).Life)
	}
	for _, f := range Unit(Spawn).Trains {
		if f == Mite {
			t.Error("Spawn shouldn't morph into Mites")
		}
	}
}
