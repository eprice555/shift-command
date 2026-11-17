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
	MinRange float64 // targets closer than this can't be hit
	Splash   float64 // radius around the target that takes SplashShare damage
}

// SplashShare is the fraction of a shot's damage dealt to other enemies in
// its splash radius. Splash never hurts friendly units.
const SplashShare = 0.5

// DamageTo returns the damage w deals to a unit with def d.
func (w Weapon) DamageTo(d UnitDef) float64 {
	return max(MinDamage, w.Damage*DamageMultiplier(w.Type, d.ArmorType)-d.Armor)
}

// UnitType identifies a unit or building definition. Buildings share the
// type space (and the sim's entity list) so they can be selected, attacked,
// and drawn through the same paths as units.
type UnitType uint8

const (
	Trooper UnitType = iota
	Outrider
	Strider
	Rigger
	Mender
	Bastion
	CommandHub
	Generator
	Extractor
	DrillYard
	Foundry

	UnitTypeCount
)

// MaxSupply is the hard supply cap per player.
const MaxSupply = 100

// UnitDef is the data-driven stat block for a unit or building. Distances
// are in tiles, speeds in tiles per second, times in seconds. Balance
// changes go here, not in sim logic.
type UnitDef struct {
	Type    UnitType
	Faction ID
	Name    string
	Role    string

	Crystals, Flux int
	Supply         float64 // supply used
	BuildTime      float64 // train time for units, construction time for buildings

	HP        float64
	Armor     float64
	ArmorType ArmorType
	Radius    float64
	Speed     float64 // 0 for buildings
	Sight     float64
	Weapon    Weapon // zero Damage means unarmed

	Bio  bool // can be healed (Mender)
	Mech bool // can be repaired (Rigger)

	Worker    bool    // gathers, constructs, repairs
	HealRate  float64 // HP per second to a nearby damaged Bio ally
	HealRange float64
	HealAura  bool // heal every damaged Bio ally in HealRange at once

	// Shift is the unit's SHIFT mode, or nil if it has none.
	Shift *Mode

	// Buildings.
	Building       bool
	Size           int        // square footprint in tiles
	SupplyProvided float64    // supply cap granted once complete
	DropOff        bool       // workers return cargo here
	OnVent         bool       // must be placed on a flux vent
	Trains         []UnitType // units this building produces
	Requires       []UnitType // completed buildings needed to build/train this
}

// Armed reports whether the def has a weapon.
func (d UnitDef) Armed() bool { return d.Weapon.Damage > 0 }

// Mode is a Forge unit's "gear shift": an alternate operating mode toggled
// with SHIFT. While it is on, the sim swaps the unit's def for Shifted(), so
// every stat lookup sees the mode without special cases.
type Mode struct {
	Name, Base string  // shifted and base mode names, e.g. "Artillery" / "Tank"
	Desc       string  // one-line effect summary for tooltips
	Time       float64 // seconds to switch either way; the unit can't act meanwhile
	Duration   float64 // > 0: the mode wears off after this many seconds
	HPCost     float64 // HP paid each time the mode is switched on

	SpeedMul float64 // movement speed multiplier (0 = unchanged)
	RateMul  float64 // fire rate multiplier (0 = unchanged)
	RangeAdd float64 // weapon range bonus
	Weapon   *Weapon // replaces the weapon
	Immobile bool    // can't move
	Unarmed  bool    // can't attack

	AuraRange, AuraRate float64 // heal aura: radius and HP per second per ally

	// Buildings: lift off and fly at FlySpeed (no production or drop-off).
	Pack     bool
	FlySpeed float64
}

// Timed reports whether the mode wears off on its own.
func (m *Mode) Timed() bool { return m.Duration > 0 }

// Shifted returns d with its SHIFT mode applied (d itself if it has none).
func (d UnitDef) Shifted() UnitDef {
	m := d.Shift
	if m == nil {
		return d
	}
	s := d
	if m.Weapon != nil {
		s.Weapon = *m.Weapon
	}
	if m.SpeedMul > 0 {
		s.Speed *= m.SpeedMul
	}
	if m.RateMul > 0 {
		s.Weapon.Cooldown /= m.RateMul
	}
	if s.Armed() {
		s.Weapon.Range += m.RangeAdd
	}
	if m.Immobile {
		s.Speed = 0
	}
	if m.Unarmed {
		s.Weapon = Weapon{}
	}
	if m.AuraRange > 0 {
		s.HealAura, s.HealRange, s.HealRate = true, m.AuraRange, m.AuraRate
	}
	if m.Pack {
		s.Speed, s.Trains, s.DropOff = m.FlySpeed, nil, false
	}
	return s
}

// Placeholder stats: costs, supply, roles, and armor classes follow the GDD;
// HP, damage, and times are first-pass values for playtesting.
var unitDefs = [UnitTypeCount]UnitDef{
	Trooper: {
		Type: Trooper, Faction: Forge, Name: "Trooper", Role: "Basic ranged infantry",
		Crystals: 50, Supply: 1, BuildTime: 14,
		HP: 45, ArmorType: Light, Radius: 0.36, Speed: 2.8, Sight: 8, Bio: true,
		Weapon: Weapon{Damage: 6, Type: Normal, Range: 5, Cooldown: 0.85},
		Shift:  &Mode{Name: "Overdrive", Desc: "+50% move and attack speed for 8s · costs 10 HP", Base: "Normal", Duration: 8, HPCost: 10, SpeedMul: 1.5, RateMul: 1.5},
	},
	Outrider: {
		Type: Outrider, Faction: Forge, Name: "Outrider", Role: "Fast raider, bonus vs Light",
		Crystals: 75, Supply: 2, BuildTime: 18,
		HP: 65, ArmorType: Light, Radius: 0.45, Speed: 4.2, Sight: 8, Mech: true,
		Weapon: Weapon{Damage: 9, Type: Concussive, Range: 2, Cooldown: 1.0},
		Shift:  &Mode{Name: "Boost", Desc: "Double speed, but can't attack", Base: "Cruise", SpeedMul: 2, Unarmed: true},
	},
	Strider: {
		Type: Strider, Faction: Forge, Name: "Strider", Role: "Anti-armor walker",
		Crystals: 100, Flux: 25, Supply: 2, BuildTime: 24,
		HP: 140, Armor: 1, ArmorType: Armored, Radius: 0.6, Speed: 2.2, Sight: 9, Mech: true,
		Weapon: Weapon{Damage: 18, Type: Explosive, Range: 6, Cooldown: 1.5},
		Shift:  &Mode{Name: "Planted", Desc: "+3 range, can't move", Base: "Walk", Time: 1, Immobile: true, RangeAdd: 3},
	},
	Rigger: {
		Type: Rigger, Faction: Forge, Name: "Rigger", Role: "Worker: gathers, builds, repairs",
		Crystals: 50, Supply: 1, BuildTime: 12,
		HP: 45, ArmorType: Light, Radius: 0.34, Speed: 2.8, Sight: 7, Bio: true, Worker: true,
		Weapon: Weapon{Damage: 5, Type: Normal, Range: 0.2, Cooldown: 1.2},
	},
	Mender: {
		Type: Mender, Faction: Forge, Name: "Mender", Role: "Support: heals infantry",
		Crystals: 50, Flux: 25, Supply: 1, BuildTime: 18,
		HP: 60, ArmorType: Light, Radius: 0.36, Speed: 2.8, Sight: 9, Bio: true,
		HealRate: 9, HealRange: 3,
		Shift: &Mode{Name: "Field Post", Desc: "Heals all infantry within 4 at once, can't move", Base: "Mobile", Time: 1, Immobile: true, AuraRange: 4, AuraRate: 6},
	},
	Bastion: {
		Type: Bastion, Faction: Forge, Name: "Bastion", Role: "Siege tank",
		Crystals: 150, Flux: 100, Supply: 3, BuildTime: 32,
		HP: 170, Armor: 1, ArmorType: Armored, Radius: 0.75, Speed: 2.2, Sight: 10, Mech: true,
		Weapon: Weapon{Damage: 15, Type: Explosive, Range: 7, Cooldown: 1.7},
		Shift: &Mode{Name: "Artillery", Desc: "Range 13 splash shells (min range 2), can't move", Base: "Tank", Time: 2, Immobile: true,
			Weapon: &Weapon{Damage: 30, Type: Explosive, Range: 13, MinRange: 2, Splash: 1.25, Cooldown: 2.8}},
	},
	CommandHub: {
		Type: CommandHub, Faction: Forge, Name: "Command Hub", Role: "HQ: trains Riggers, cargo drop-off",
		Crystals: 400, BuildTime: 60,
		HP: 1500, Armor: 1, ArmorType: Armored, Radius: 2, Sight: 10, Mech: true,
		Building: true, Size: 4, SupplyProvided: 10, DropOff: true, Trains: []UnitType{Rigger},
		Shift: &Mode{Name: "Packed", Desc: "Pack up and fly to a new site; SHIFT again to land", Base: "Landed", Time: 1.5, Pack: true, FlySpeed: 1.2},
	},
	Generator: {
		Type: Generator, Faction: Forge, Name: "Generator", Role: "+8 supply",
		Crystals: 100, BuildTime: 20,
		HP: 400, Armor: 1, ArmorType: Armored, Radius: 1, Sight: 7, Mech: true,
		Building: true, Size: 2, SupplyProvided: 8,
	},
	Extractor: {
		Type: Extractor, Faction: Forge, Name: "Extractor", Role: "Harvests flux from a vent",
		Crystals: 75, BuildTime: 20,
		HP: 450, Armor: 1, ArmorType: Armored, Radius: 1, Sight: 7, Mech: true,
		Building: true, Size: 2, OnVent: true,
	},
	DrillYard: {
		Type: DrillYard, Faction: Forge, Name: "Drill Yard", Role: "Trains Troopers, Outriders, Menders",
		Crystals: 150, BuildTime: 40,
		HP: 1000, Armor: 1, ArmorType: Armored, Radius: 1.5, Sight: 8, Mech: true,
		Building: true, Size: 3, Trains: []UnitType{Trooper, Outrider, Mender}, Requires: []UnitType{CommandHub},
		Shift: &Mode{Name: "Packed", Desc: "Pack up and fly to a new site; SHIFT again to land", Base: "Landed", Time: 1.5, Pack: true, FlySpeed: 1.4},
	},
	Foundry: {
		Type: Foundry, Faction: Forge, Name: "Foundry", Role: "Trains Striders, Bastions",
		Crystals: 150, Flux: 100, BuildTime: 45,
		HP: 1150, Armor: 1, ArmorType: Armored, Radius: 1.5, Sight: 8, Mech: true,
		Building: true, Size: 3, Trains: []UnitType{Strider, Bastion}, Requires: []UnitType{DrillYard},
		Shift: &Mode{Name: "Packed", Desc: "Pack up and fly to a new site; SHIFT again to land", Base: "Landed", Time: 1.5, Pack: true, FlySpeed: 1.4},
	},
}

// Unit returns the definition for t. It panics on an out-of-range type.
func Unit(t UnitType) UnitDef { return unitDefs[t] }

// Units returns every unit and building definition in type order.
func Units() []UnitDef { return unitDefs[:] }

// Buildable returns the building types a worker of faction f can construct,
// in command-card order.
func Buildable(f ID) []UnitType {
	var out []UnitType
	for _, d := range unitDefs {
		if d.Building && d.Faction == f {
			out = append(out, d.Type)
		}
	}
	return out
}

// TrainedAt returns the building type that trains t, or false if none does.
func TrainedAt(t UnitType) (UnitType, bool) {
	for _, d := range unitDefs {
		for _, x := range d.Trains {
			if x == t {
				return d.Type, true
			}
		}
	}
	return 0, false
}
