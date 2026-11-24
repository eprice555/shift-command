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
	VsWorker float64 // bonus damage against workers
}

// SplashShare is the fraction of a shot's damage dealt to other enemies in
// its splash radius. Splash never hurts friendly units.
const SplashShare = 0.5

// DamageTo returns the damage w deals to a unit with def d.
func (w Weapon) DamageTo(d UnitDef) float64 {
	dmg := w.Damage
	if d.Worker {
		dmg += w.VsWorker
	}
	return max(MinDamage, dmg*DamageMultiplier(w.Type, d.ArmorType)-d.Armor)
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

	// The Brood.
	Spawn
	Drone
	Biter
	Leaper
	Spitter
	Broodmother
	Behemoth
	Mite
	Hive
	NestPod
	BroodExtractor
	SpawningPit
	GestationDen

	// The Phasebound.
	Drifter
	Warden
	Glint
	Seer
	Conduit
	Monolith
	Spire
	Anchor
	PhaseExtractor
	Gate
	Resonator
	Radiant

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
	Regen     float64 // HP per second regenerated while complete (Brood)
	HealRate  float64 // HP per second to a nearby damaged Bio ally
	HealRange float64
	HealAura  bool // heal every damaged Bio ally in HealRange at once
	// ShieldHeal makes HealRate restore allies' shields instead of HP.
	ShieldHeal bool

	// Shield is the Phasebound's regenerating shield pool, which absorbs
	// damage before HP. It recharges after ShieldDelay seconds unhurt.
	Shield float64

	// Shift is the unit's SHIFT mode, or nil if it has none.
	Shift *Mode

	Burrowed bool    // hidden from enemies (a SHIFT mode sets it)
	Life     float64 // > 0: a summoned unit that expires after this many seconds

	// Brood morphing. A Morphs unit (Spawn) turns into one of its Trains
	// forms in place; Spawns is how many units one morph yields (Biters
	// hatch in pairs; 0 means 1). Cost is per morph, Supply per unit.
	Morphs bool
	Spawns int

	// Buildings.
	Building       bool
	Size           int        // square footprint in tiles
	SupplyProvided float64    // supply cap granted once complete
	DropOff        bool       // workers return cargo here
	OnVent         bool       // must be placed on a flux vent
	Trains         []UnitType // units this building produces
	Requires       []UnitType // completed buildings needed to build/train this

	SelfBuild  bool    // the worker is consumed and the building grows on its own (Brood)
	NeedsCreep bool    // the whole footprint must be on its owner's creep
	Creep      float64 // creep radius spread once complete
	SpawnMax   int     // a Hive keeps up to this many idle Spawn

	// Warped buildings are summoned: the worker starts one and walks away
	// while it builds itself (Phasebound). Requires SelfBuild.
	Warped      bool
	NeedsPower  bool    // must be placed (and stay) in an Anchor's power field
	PowerRadius float64 // power field radius once complete
}

// Yield is how many units one train/morph of d produces.
func (d UnitDef) Yield() int { return max(1, d.Spawns) }

// QueueSupply is the supply one train/morph of t reserves.
func QueueSupply(t UnitType) float64 {
	d := Unit(t)
	return d.Supply * float64(d.Yield())
}

// Armed reports whether the def has a weapon.
func (d UnitDef) Armed() bool { return d.Weapon.Damage > 0 }

// Mode is a unit's SHIFT ability. Most are alternate operating modes
// toggled with SHIFT (the Forge's gear shifts, the Brood's Burrow, Root,
// and Shell): while one is on, the sim swaps the unit's def for Shifted(),
// so every stat lookup sees the mode without special cases. A mode with a
// Cooldown is instead an instant ability cast with SHIFT (Pounce, Brood
// Burst, and Phase).
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
	ArmorAdd float64 // armor bonus
	Burrow   bool    // hidden from enemies; can't move or attack

	AuraRange, AuraRate float64 // heal aura: radius and HP per second per ally

	// Buildings: lift off and fly at FlySpeed (no production or drop-off).
	Pack     bool
	FlySpeed float64

	// Instant abilities.
	Cooldown      float64  // > 0: cast on SHIFT, then wait this many seconds
	Leap          float64  // leap at an enemy up to this far away (tiles)
	LeapDamage    float64  // damage dealt on landing
	Summon        UnitType // summon SummonCount of this type around the caster
	SummonCount   int
	PhaseDuration float64 // phase out for this many seconds
	PhaseRadius   float64 // radius of a group phase cast
	DashRange     float64 // dash toward a visible enemy within this range
	BlinkRange    float64 // blink forward by this distance
	LongPhase     bool    // phase movement ignores terrain
}

// Cast reports whether the mode is an instant ability rather than a toggle.
func (m *Mode) Cast() bool { return m.Cooldown > 0 }

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
	if m.Unarmed || m.Burrow {
		s.Weapon = Weapon{}
	}
	if m.Burrow {
		s.Speed, s.Burrowed = 0, true
	}
	s.Armor += m.ArmorAdd
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

	Spawn: {
		Type: Spawn, Faction: Brood, Name: "Spawn", Role: "Larva: morphs into any unlocked Brood form",
		BuildTime: SpawnTime,
		HP:        25, Armor: 6, ArmorType: Light, Radius: 0.26, Sight: 4, Bio: true, Regen: broodUnitRegen,
		Morphs: true, Trains: broodForms,
	},
	Drone: {
		Type: Drone, Faction: Brood, Name: "Drone", Role: "Worker: gathers, morphs into buildings",
		Crystals: 50, Supply: 1, BuildTime: 12,
		HP: 40, ArmorType: Light, Radius: 0.34, Speed: 2.8, Sight: 7, Bio: true, Worker: true, Regen: broodUnitRegen,
		Weapon: Weapon{Damage: 5, Type: Normal, Range: 0.2, Cooldown: 1.2},
	},
	Biter: {
		Type: Biter, Faction: Brood, Name: "Biter", Role: "Fast melee swarm (hatch in pairs)",
		Crystals: 50, Supply: 0.5, BuildTime: 16, Spawns: 2,
		HP: 35, ArmorType: Light, Radius: 0.28, Speed: 4, Sight: 7, Bio: true, Regen: broodUnitRegen,
		Weapon:   Weapon{Damage: 5, Type: Normal, Range: 0.15, Cooldown: 0.7},
		Requires: []UnitType{SpawningPit},
		Shift:    &Mode{Name: "Burrowed", Desc: "Hide underground: invisible to enemies, can't move or attack", Base: "Surface", Time: 0.75, Burrow: true},
	},
	Leaper: {
		Type: Leaper, Faction: Brood, Name: "Leaper", Role: "Raider: hard-hitting melee",
		Crystals: 50, Flux: 25, Supply: 1, BuildTime: 18,
		HP: 55, ArmorType: Light, Radius: 0.36, Speed: 3.6, Sight: 8, Bio: true, Regen: broodUnitRegen,
		Weapon:   Weapon{Damage: 9, Type: Normal, Range: 0.25, Cooldown: 0.9},
		Requires: []UnitType{SpawningPit},
		Shift:    &Mode{Name: "Pounce", Desc: "Leap onto an enemy up to 5 tiles away for 12 damage · 8s cooldown", Cooldown: 8, Leap: 5, LeapDamage: 12},
	},
	Spitter: {
		Type: Spitter, Faction: Brood, Name: "Spitter", Role: "Ranged acid",
		Crystals: 75, Flux: 25, Supply: 2, BuildTime: 22,
		HP: 85, ArmorType: Light, Radius: 0.45, Speed: 2.6, Sight: 8, Bio: true, Regen: broodUnitRegen,
		Weapon:   Weapon{Damage: 11, Type: Normal, Range: 5, Cooldown: 1.2},
		Requires: []UnitType{SpawningPit},
		Shift:    &Mode{Name: "Rooted", Desc: "+2 range, can't move", Base: "Mobile", Time: 1, Immobile: true, RangeAdd: 2},
	},
	Broodmother: {
		Type: Broodmother, Faction: Brood, Name: "Broodmother", Role: "Support: spits and transfuses HP to the brood",
		Crystals: 100, Flux: 100, Supply: 2, BuildTime: 30,
		HP: 160, Armor: 1, ArmorType: Armored, Radius: 0.6, Speed: 2.4, Sight: 9, Bio: true, Regen: broodUnitRegen,
		Weapon:   Weapon{Damage: 8, Type: Normal, Range: 4, Cooldown: 1.2},
		HealRate: 6, HealRange: 3,
		Requires: []UnitType{GestationDen},
		Shift:    &Mode{Name: "Brood Burst", Desc: "Spawn 4 Mites that live for 10s · 25s cooldown", Cooldown: 25, Summon: Mite, SummonCount: 4},
	},
	Behemoth: {
		Type: Behemoth, Faction: Brood, Name: "Behemoth", Role: "Heavy melee tank",
		Crystals: 200, Flux: 100, Supply: 4, BuildTime: 38,
		HP: 400, Armor: 2, ArmorType: Armored, Radius: 0.85, Speed: 2.4, Sight: 8, Bio: true, Regen: broodUnitRegen,
		Weapon:   Weapon{Damage: 25, Type: Normal, Range: 0.4, Cooldown: 1.4},
		Requires: []UnitType{GestationDen},
		Shift:    &Mode{Name: "Shell", Desc: "+4 armor, half speed", Base: "Open", Time: 0.5, ArmorAdd: 4, SpeedMul: 0.5},
	},
	Mite: {
		Type: Mite, Faction: Brood, Name: "Mite", Role: "Short-lived swarmer from a Brood Burst",
		HP: 20, ArmorType: Light, Radius: 0.22, Speed: 4.2, Sight: 6, Bio: true, Regen: broodUnitRegen,
		Weapon: Weapon{Damage: 4, Type: Normal, Range: 0.15, Cooldown: 0.6},
		Life:   10,
	},
	Hive: {
		Type: Hive, Faction: Brood, Name: "Hive", Role: "HQ: hatches Spawn, spreads creep, cargo drop-off",
		Crystals: 300, BuildTime: 60,
		HP: 1500, Armor: 1, ArmorType: Armored, Radius: 2, Sight: 10, Regen: broodBuildingRegen,
		Building: true, Size: 4, SupplyProvided: 10, DropOff: true, Trains: broodForms,
		SelfBuild: true, Creep: 9, SpawnMax: 3,
	},
	NestPod: {
		Type: NestPod, Faction: Brood, Name: "Nest Pod", Role: "+8 supply, spreads creep",
		Crystals: 100, BuildTime: 20,
		HP: 400, Armor: 1, ArmorType: Armored, Radius: 1, Sight: 7, Regen: broodBuildingRegen,
		Building: true, Size: 2, SupplyProvided: 8,
		SelfBuild: true, NeedsCreep: true, Creep: 5,
	},
	BroodExtractor: {
		Type: BroodExtractor, Faction: Brood, Name: "Extractor", Role: "Harvests flux from a vent",
		Crystals: 25, BuildTime: 20,
		HP: 450, Armor: 1, ArmorType: Armored, Radius: 1, Sight: 7, Regen: broodBuildingRegen,
		Building: true, Size: 2, OnVent: true, SelfBuild: true,
	},
	SpawningPit: {
		Type: SpawningPit, Faction: Brood, Name: "Spawning Pit", Role: "Unlocks Biters, Leapers, Spitters",
		Crystals: 200, BuildTime: 40,
		HP: 1000, Armor: 1, ArmorType: Armored, Radius: 1.5, Sight: 8, Regen: broodBuildingRegen,
		Building: true, Size: 3, Requires: []UnitType{Hive},
		SelfBuild: true, NeedsCreep: true, Creep: 3,
	},
	GestationDen: {
		Type: GestationDen, Faction: Brood, Name: "Gestation Den", Role: "Unlocks Broodmothers, Behemoths",
		Crystals: 150, Flux: 100, BuildTime: 45,
		HP: 1000, Armor: 1, ArmorType: Armored, Radius: 1.5, Sight: 8, Regen: broodBuildingRegen,
		Building: true, Size: 3, Requires: []UnitType{SpawningPit},
		SelfBuild: true, NeedsCreep: true, Creep: 3,
	},

	Drifter: {
		Type: Drifter, Faction: Phasebound, Name: "Drifter", Role: "Worker: gathers, summons buildings",
		Crystals: 50, Supply: 1, BuildTime: 12,
		HP: 25, Shield: 25, ArmorType: Light, Radius: 0.34, Speed: 2.8, Sight: 7, Worker: true,
		Weapon: Weapon{Damage: 5, Type: Normal, Range: 0.2, Cooldown: 1.2},
		Shift:  &Mode{Name: "Phase", Desc: "Become untouchable and pass through units for 2s · 10s cooldown", Cooldown: 10, PhaseDuration: 2},
	},
	Warden: {
		Type: Warden, Faction: Phasebound, Name: "Warden", Role: "Shielded melee warrior",
		Crystals: 100, Supply: 2, BuildTime: 24,
		HP: 90, Shield: 60, Armor: 1, ArmorType: Light, Radius: 0.45, Speed: 2.9, Sight: 8,
		Weapon: Weapon{Damage: 9, Type: Normal, Range: 0.3, Cooldown: 1.0},
		Shift:  &Mode{Name: "Phase Dash", Desc: "Dash to a nearby enemy while phased for 2s · 10s cooldown", Cooldown: 10, PhaseDuration: 2, DashRange: 4.5},
	},
	Glint: {
		Type: Glint, Faction: Phasebound, Name: "Glint", Role: "Fast raider and scout, bonus vs workers",
		Crystals: 100, Flux: 25, Supply: 2, BuildTime: 22,
		HP: 60, Shield: 40, ArmorType: Light, Radius: 0.4, Speed: 4.5, Sight: 10,
		Weapon: Weapon{Damage: 6, Type: Normal, Range: 1.5, Cooldown: 0.8, VsWorker: 8},
		Shift:  &Mode{Name: "Long Phase", Desc: "Become untouchable, pass through units and cliffs for 4s · 10s cooldown", Cooldown: 10, PhaseDuration: 4, LongPhase: true},
	},
	Seer: {
		Type: Seer, Faction: Phasebound, Name: "Seer", Role: "Ranged psionic bolts, bonus vs Armored",
		Crystals: 125, Flux: 50, Supply: 2, BuildTime: 28,
		HP: 50, Shield: 70, ArmorType: Light, Radius: 0.42, Speed: 2.6, Sight: 9,
		Weapon: Weapon{Damage: 14, Type: Explosive, Range: 6, Cooldown: 1.6},
		Shift:  &Mode{Name: "Phase Blink", Desc: "Blink forward and become untouchable for 2s · 10s cooldown", Cooldown: 10, PhaseDuration: 2, BlinkRange: 4},
	},
	Conduit: {
		Type: Conduit, Faction: Phasebound, Name: "Conduit", Role: "Support: restores allies' shields",
		Crystals: 50, Flux: 150, Supply: 2, BuildTime: 30,
		HP: 80, Shield: 80, Armor: 1, ArmorType: Armored, Radius: 0.5, Speed: 2.6, Sight: 9,
		HealRate: 12, HealRange: 4, ShieldHeal: true,
		Shift: &Mode{Name: "Phase Field", Desc: "Phase nearby allies for 3s · 30s cooldown", Cooldown: 30, PhaseDuration: 3, PhaseRadius: 4},
	},
	Monolith: {
		Type: Monolith, Faction: Phasebound, Name: "Monolith", Role: "Heavy ranged siege",
		Crystals: 300, Flux: 200, Supply: 6, BuildTime: 50,
		HP: 220, Shield: 180, Armor: 2, ArmorType: Armored, Radius: 0.9, Speed: 1.8, Sight: 10,
		Weapon: Weapon{Damage: 32, Type: Explosive, Range: 9, Cooldown: 2.6, Splash: 1},
		Shift:  &Mode{Name: "Phase", Desc: "Become untouchable and pass through units for 2s · 10s cooldown", Cooldown: 10, PhaseDuration: 2},
	},
	Spire: {
		Type: Spire, Faction: Phasebound, Name: "Spire", Role: "HQ: trains Drifters, cargo drop-off",
		Crystals: 400, BuildTime: 60,
		HP: 1000, Shield: 1000, Armor: 1, ArmorType: Armored, Radius: 2, Sight: 10,
		Building: true, Size: 4, SupplyProvided: 10, DropOff: true, Trains: []UnitType{Drifter},
		SelfBuild: true, Warped: true,
	},
	Anchor: {
		Type: Anchor, Faction: Phasebound, Name: "Anchor", Role: "+8 supply, powers nearby buildings",
		Crystals: 100, BuildTime: 18,
		HP: 200, Shield: 200, Armor: 1, ArmorType: Armored, Radius: 1, Sight: 8,
		Building: true, Size: 2, SupplyProvided: 8,
		SelfBuild: true, Warped: true, PowerRadius: 6.5,
	},
	PhaseExtractor: {
		Type: PhaseExtractor, Faction: Phasebound, Name: "Extractor", Role: "Harvests flux from a vent",
		Crystals: 75, BuildTime: 20,
		HP: 450, Shield: 450, Armor: 1, ArmorType: Armored, Radius: 1, Sight: 7,
		Building: true, Size: 2, OnVent: true, SelfBuild: true, Warped: true,
	},
	Gate: {
		Type: Gate, Faction: Phasebound, Name: "Gate", Role: "Trains Wardens, Glints, Seers",
		Crystals: 150, BuildTime: 40,
		HP: 500, Shield: 500, Armor: 1, ArmorType: Armored, Radius: 1.5, Sight: 8,
		Building: true, Size: 3, Trains: []UnitType{Warden, Glint, Seer}, Requires: []UnitType{Anchor},
		SelfBuild: true, Warped: true, NeedsPower: true,
	},
	Resonator: {
		Type: Resonator, Faction: Phasebound, Name: "Resonator", Role: "Trains Conduits, Monoliths",
		Crystals: 150, Flux: 150, BuildTime: 45,
		HP: 550, Shield: 550, Armor: 1, ArmorType: Armored, Radius: 1.5, Sight: 8,
		Building: true, Size: 3, Trains: []UnitType{Conduit, Monolith}, Requires: []UnitType{Gate},
		SelfBuild: true, Warped: true, NeedsPower: true,
	},
	Radiant: {
		Type: Radiant, Faction: Phasebound, Name: "Radiant", Role: "Fused Seers: high-shield psionic artillery",
		Supply: 4,
		HP:     100, Shield: 250, Armor: 2, ArmorType: Armored, Radius: 0.75, Speed: 2.2, Sight: 9,
		Weapon:   Weapon{Damage: 24, Type: Explosive, Range: 3.5, Cooldown: 1.5, Splash: 1.5},
		Requires: []UnitType{Resonator},
		Shift:    &Mode{Name: "Phase", Desc: "Become untouchable and pass through units for 2s · 10s cooldown", Cooldown: 10, PhaseDuration: 2},
	},
}

// Brood tuning.
const (
	// SpawnTime is how often (seconds) a Hive hatches a Spawn while it has
	// fewer than SpawnMax idle ones.
	SpawnTime          = 10
	broodUnitRegen     = 0.6
	broodBuildingRegen = 2
)

// Phasebound tuning.
const (
	// ShieldDelay is how long (seconds) shields wait after damage before
	// they recharge at ShieldRegen per second.
	ShieldDelay          = 7
	ShieldRegen          = 2
	WarpResearchCrystals = 150
	WarpResearchFlux     = 150
	WarpResearchTime     = 60
)

// broodForms is what a Spawn can morph into, in command-card order.
var broodForms = []UnitType{Drone, Biter, Leaper, Spitter, Broodmother, Behemoth}

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
		if !d.Building {
			continue
		}
		for _, x := range d.Trains {
			if x == t {
				return d.Type, true
			}
		}
	}
	return 0, false
}
