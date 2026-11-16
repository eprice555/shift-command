package faction

// ArmorType is a unit's armor class, which attack types scale against.
type ArmorType uint8

const (
	Light ArmorType = iota
	Armored
)

func (a ArmorType) String() string {
	if a == Armored {
		return "Armored"
	}
	return "Light"
}

// AttackType determines bonus or reduced damage by armor type.
type AttackType uint8

const (
	Normal     AttackType = iota // full damage to everything
	Concussive                   // good vs Light, poor vs Armored
	Explosive                    // good vs Armored, poor vs Light
)

func (a AttackType) String() string {
	switch a {
	case Concussive:
		return "Concussive"
	case Explosive:
		return "Explosive"
	}
	return "Normal"
}

var damageTable = [3][2]float64{
	Normal:     {Light: 1, Armored: 1},
	Concussive: {Light: 1.25, Armored: 0.6},
	Explosive:  {Light: 0.6, Armored: 1.5},
}

// DamageMultiplier returns the attack-vs-armor damage scale.
func DamageMultiplier(a AttackType, ar ArmorType) float64 { return damageTable[a][ar] }

// MinDamage is the floor applied after armor so every hit does something.
const MinDamage = 0.5

// Weapon is a unit's attack. Ranges are edge-to-edge, in tiles.
type Weapon struct {
	Damage   float64
	Type     AttackType
	Range    float64
	Cooldown float64 // seconds between shots
}

// DamageTo returns the damage w deals to a unit with def d.
func (w Weapon) DamageTo(d UnitDef) float64 {
	return max(MinDamage, w.Damage*DamageMultiplier(w.Type, d.ArmorType)-d.Armor)
}

// UnitType identifies a unit definition.
type UnitType uint8

const (
	Trooper UnitType = iota
	Outrider
	Strider

	UnitTypeCount
)

// UnitDef is the data-driven stat block for a unit. Distances are in tiles,
// speeds in tiles per second. Balance changes go here, not in sim logic.
type UnitDef struct {
	Type    UnitType
	Faction ID
	Name    string
	Role    string

	Crystals, Flux int
	Supply         float64

	HP        float64
	Armor     float64
	ArmorType ArmorType
	Radius    float64
	Speed     float64
	Sight     float64
	Weapon    Weapon
}

// M1 placeholder stats for the first Forge units. Costs and supply follow the
// GDD; combat numbers are first-pass values for the M1 playtest.
var unitDefs = [UnitTypeCount]UnitDef{
	Trooper: {
		Type: Trooper, Faction: Forge, Name: "Trooper", Role: "Basic ranged infantry",
		Crystals: 50, Supply: 1,
		HP: 45, ArmorType: Light, Radius: 0.36, Speed: 2.8, Sight: 8,
		Weapon: Weapon{Damage: 6, Type: Normal, Range: 5, Cooldown: 0.85},
	},
	Outrider: {
		Type: Outrider, Faction: Forge, Name: "Outrider", Role: "Fast raider, bonus vs Light",
		Crystals: 75, Supply: 2,
		HP: 65, ArmorType: Light, Radius: 0.45, Speed: 4.2, Sight: 8,
		Weapon: Weapon{Damage: 9, Type: Concussive, Range: 2, Cooldown: 1.0},
	},
	Strider: {
		Type: Strider, Faction: Forge, Name: "Strider", Role: "Anti-armor walker",
		Crystals: 100, Flux: 25, Supply: 2,
		HP: 140, Armor: 1, ArmorType: Armored, Radius: 0.6, Speed: 2.2, Sight: 9,
		Weapon: Weapon{Damage: 18, Type: Explosive, Range: 6, Cooldown: 1.5},
	},
}

// Unit returns the definition for t. It panics on an out-of-range type.
func Unit(t UnitType) UnitDef { return unitDefs[t] }

// Units returns every unit definition in type order.
func Units() []UnitDef { return unitDefs[:] }
