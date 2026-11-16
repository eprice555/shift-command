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
		if d.Name == "" || d.HP <= 0 || d.Radius <= 0 || d.Speed <= 0 || d.Sight <= 0 ||
			d.Weapon.Damage <= 0 || d.Weapon.Range <= 0 || d.Weapon.Cooldown <= 0 {
			t.Errorf("%s: bad stats %+v", d.Name, d)
		}
		if d.Sight < d.Weapon.Range {
			t.Errorf("%s: sight %v below weapon range %v", d.Name, d.Sight, d.Weapon.Range)
		}
	}
}
