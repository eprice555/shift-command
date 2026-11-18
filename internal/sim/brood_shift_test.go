package sim

import (
	"slices"
	"testing"

	"github.com/eprice555/shift-command/internal/faction"
	"github.com/eprice555/shift-command/internal/geom"
)

func morphCmd(p PlayerID, t faction.UnitType, us ...*Unit) Command {
	return Command{Kind: CmdMorph, Player: p, Units: ids(us), Type: t}
}

func TestRootedSpitterOutranges(t *testing.T) {
	w := openWorld(30, 20)
	s := shifted(w, 0, faction.Spitter, geom.V(5, 10))
	if s.Def.Weapon.Range != 7 || s.Def.Speed != 0 {
		t.Fatalf("rooted: range %.0f speed %.1f", s.Def.Weapon.Range, s.Def.Speed)
	}
	// 6.5 tiles edge to edge: out of base range, inside rooted range.
	e := w.Spawn(1, faction.Rigger, geom.V(5+s.Def.Radius+6.5+faction.Unit(faction.Rigger).Radius, 10))
	e.Hold = true
	run(w, secs(3))
	if e.HP == e.Def.HP {
		t.Error("rooted Spitter didn't hit a target at 6.5 tiles")
	}
	start := s.Pos
	run(w, secs(1), Command{Kind: CmdMove, Units: []UnitID{s.ID}, Point: geom.V(20, 5)})
	if s.Pos != start {
		t.Error("rooted Spitter moved")
	}
}

func TestShellArmor(t *testing.T) {
	w := openWorld(30, 20)
	b := shifted(w, 0, faction.Behemoth, geom.V(5, 10))
	base := faction.Unit(faction.Behemoth)
	if b.Def.Armor != base.Armor+4 || b.Def.Speed != base.Speed/2 || !b.Def.Armed() {
		t.Fatalf("shell: armor %.0f speed %.1f", b.Def.Armor, b.Def.Speed)
	}
	if got, want := faction.Unit(faction.Trooper).Weapon.DamageTo(b.Def), faction.Unit(faction.Trooper).Weapon.DamageTo(base); got >= want {
		t.Errorf("shelled Behemoth takes %.1f per Trooper shot, open takes %.1f", got, want)
	}
}

func TestBurrowHidesFromEnemies(t *testing.T) {
	w := openWorld(30, 20)
	b := w.Spawn(0, faction.Biter, geom.V(10, 10))
	e := w.Spawn(1, faction.Trooper, geom.V(14, 10))
	e.Hold = true
	run(w, 1, shiftCmd(0, -1, b))
	if !b.Shifting() || b.Hidden() || !w.VisibleTo(1, b) {
		t.Fatal("a Biter should stay visible while it digs in")
	}
	run(w, secs(1))
	if !b.Hidden() || w.VisibleTo(1, b) || !w.VisibleTo(0, b) {
		t.Fatalf("burrowed: hidden %v, enemy sees it %v", b.Hidden(), w.VisibleTo(1, b))
	}
	hp := b.HP
	run(w, secs(3), Command{Kind: CmdAttack, Player: 1, Units: []UnitID{e.ID}, Target: b.ID})
	if b.HP < hp || e.Target() != 0 {
		t.Fatalf("enemy found a burrowed Biter: hp %.1f -> %.1f, target %d", hp, b.HP, e.Target())
	}
	start := b.Pos
	run(w, secs(1), Command{Kind: CmdMove, Units: []UnitID{b.ID}, Point: geom.V(3, 3)})
	if b.Pos != start {
		t.Error("burrowed Biter moved")
	}
	// Surfacing reveals it again.
	run(w, secs(2), shiftCmd(0, -1, b))
	if b.Hidden() || !w.VisibleTo(1, b) || b.Def.Speed == 0 {
		t.Fatal("unburrowed Biter still hidden")
	}
}

func TestPounce(t *testing.T) {
	w := openWorld(30, 20)
	l := w.Spawn(0, faction.Leaper, geom.V(5, 10))
	far := w.Spawn(1, faction.Rigger, geom.V(25, 10))
	if n := notices(w, 1, shiftCmd(0, -1, l)); !slices.Contains(n, NoticeNoTarget) || l.CastLeft() != 0 {
		t.Fatalf("pounce with nothing in reach: notices %v, cooldown %.1f", n, l.CastLeft())
	}
	e := w.Spawn(1, faction.Rigger, geom.V(9.5, 10))
	e.Hold = true
	run(w, 1, shiftCmd(0, -1, l))
	if !l.Leaping() || l.CastLeft() <= 0 {
		t.Fatalf("leaping %v cooldown %.1f", l.Leaping(), l.CastLeft())
	}
	run(w, secs(0.5))
	if l.Leaping() || gap(l, e) > l.Def.Weapon.Range+0.1 {
		t.Fatalf("leap didn't land: leaping %v gap %.2f", l.Leaping(), gap(l, e))
	}
	if e.HP > e.Def.HP-faction.Unit(faction.Leaper).Shift.LeapDamage {
		t.Errorf("landing hit missing: hp %.1f", e.HP)
	}
	if l.Target() != e.ID {
		t.Errorf("Leaper should keep fighting its pounce target, has %d", l.Target())
	}
	if n := notices(w, 1, shiftCmd(0, -1, l)); !slices.Contains(n, NoticeCooldown) {
		t.Errorf("recast during cooldown: notices %v", n)
	}
	_ = far
}

func TestBroodBurstMitesExpire(t *testing.T) {
	w := openWorld(30, 20)
	m := w.Spawn(0, faction.Broodmother, geom.V(10, 10))
	supply := 0.0
	run(w, 1)
	supply = w.Players[0].Supply
	run(w, 1, shiftCmd(0, -1, m))
	mites := find(w, 0, faction.Mite)
	if len(mites) != 4 || w.Players[0].Supply != supply {
		t.Fatalf("%d mites, supply %v -> %v", len(mites), supply, w.Players[0].Supply)
	}
	if n := notices(w, 1, shiftCmd(0, -1, m)); !slices.Contains(n, NoticeCooldown) || len(find(w, 0, faction.Mite)) != 4 {
		t.Fatalf("burst during cooldown: notices %v", n)
	}
	run(w, secs(9.5))
	if len(find(w, 0, faction.Mite)) != 4 {
		t.Fatal("mites expired early")
	}
	run(w, secs(1))
	if n := len(find(w, 0, faction.Mite)); n != 0 {
		t.Fatalf("%d mites outlived their 10s", n)
	}
	run(w, secs(15))
	run(w, 1, shiftCmd(0, -1, m))
	if len(find(w, 0, faction.Mite)) != 4 {
		t.Fatal("burst didn't recharge after 25s")
	}
}

func TestMitesJoinTheFight(t *testing.T) {
	w := openWorld(30, 20)
	m := w.Spawn(0, faction.Broodmother, geom.V(10, 10))
	e := w.Spawn(1, faction.Rigger, geom.V(13, 10))
	e.Hold = true
	run(w, secs(1), Command{Kind: CmdAttack, Player: 0, Units: []UnitID{m.ID}, Target: e.ID})
	run(w, 1, shiftCmd(0, -1, m))
	for _, mi := range find(w, 0, faction.Mite) {
		if len(mi.Orders) == 0 || mi.Orders[0].Target != e.ID {
			t.Fatalf("mite orders %+v, want attack %d", mi.Orders, e.ID)
		}
	}
}

// broodArmy returns a Brood match with both tech buildings, plenty of
// resources, and a fresh unit of type t.
func broodArmy(t faction.UnitType) (*World, *Unit) {
	w := broodMatch()
	w.Spawn(0, faction.SpawningPit, geom.V(20, 50))
	w.Spawn(0, faction.GestationDen, geom.V(20, 56))
	w.Players[0].Crystals, w.Players[0].Flux = 1000, 1000
	u := w.Spawn(0, t, hive(w).Pos.Add(geom.V(0, -4)))
	run(w, 1)
	return w, u
}

func TestRemorph(t *testing.T) {
	w, b := broodArmy(faction.Biter)
	b.HP = b.Def.HP / 2
	before := w.Players[0]
	run(w, 1, morphCmd(0, faction.Spitter, b))
	if !b.Morphing() || !b.Remorphing() || b.Type != faction.Spawn || b.RemorphFrom() != faction.Biter {
		t.Fatalf("not cocooned: type %d morphing %v", b.Type, b.Morphing())
	}
	pl := w.Players[0]
	if c, f := RemorphCost(faction.Biter, faction.Spitter); c != 50 || f != 25 || pl.Crystals != before.Crystals-c || pl.Flux != before.Flux-f {
		t.Fatalf("cost %d/%d, paid %d/%d", c, f, before.Crystals-pl.Crystals, before.Flux-pl.Flux)
	}
	if want := before.Supply - 0.5 + 2; pl.Supply != want {
		t.Errorf("supply %v, want %v", pl.Supply, want)
	}
	// Half of the Spitter's 22s morph.
	run(w, secs(10.5))
	if !b.Morphing() {
		t.Fatal("re-morph finished early")
	}
	run(w, secs(1))
	if b.Morphing() || b.Type != faction.Spitter || len(find(w, 0, faction.Spitter)) != 1 {
		t.Fatalf("type %d morphing %v", b.Type, b.Morphing())
	}
	if b.HP >= b.Def.HP*0.8 {
		t.Errorf("re-morph healed a wounded Biter to %.0f/%.0f", b.HP, b.Def.HP)
	}
	if b.Remorphing() || w.Players[0].Supply != before.Supply-0.5+2 {
		t.Errorf("supply after hatch %v", w.Players[0].Supply)
	}
}

func TestRemorphCancelAndLimits(t *testing.T) {
	w, b := broodArmy(faction.Biter)
	before := w.Players[0]
	run(w, secs(2), morphCmd(0, faction.Leaper, b))
	run(w, 1, Command{Kind: CmdCancel, Player: 0, Units: []UnitID{b.ID}})
	pl := w.Players[0]
	if b.Morphing() || b.Type != faction.Biter || b.Def.Speed == 0 {
		t.Fatalf("cancel: type %d morphing %v", b.Type, b.Morphing())
	}
	if pl.Crystals != before.Crystals || pl.Flux != before.Flux || pl.Supply != before.Supply {
		t.Errorf("not refunded: %+v vs %+v", pl, before)
	}
	if n := notices(w, 1, morphCmd(0, faction.Biter, b)); !slices.Contains(n, NoticeSameForm) || b.Morphing() {
		t.Errorf("same form: notices %v", n)
	}
	if c, f := RemorphCost(faction.Behemoth, faction.Biter); c != 0 || f != 0 {
		t.Errorf("downgrade costs %d/%d", c, f)
	}
	if !CanRemorph(b, faction.Drone) || CanRemorph(b, faction.Mite) || CanRemorph(b, faction.Hive) {
		t.Error("CanRemorph forms")
	}

	w = broodMatch()
	w.Players[0].Crystals, w.Players[0].Flux = 1000, 1000
	d := find(w, 0, faction.Drone)[0]
	if n := notices(w, 1, morphCmd(0, faction.Spitter, d)); !slices.Contains(n, NoticeRequires) || d.Morphing() {
		t.Errorf("no Spawning Pit: notices %v", n)
	}
}

func TestRemorphResetsMode(t *testing.T) {
	w, b := broodArmy(faction.Behemoth)
	run(w, secs(1), shiftCmd(0, -1, b))
	if b.Mode != 1 {
		t.Fatal("shell didn't close")
	}
	run(w, secs(20), morphCmd(0, faction.Leaper, b))
	if b.Type != faction.Leaper || b.Mode != 0 || b.Def.Speed != faction.Unit(faction.Leaper).Speed {
		t.Fatalf("type %d mode %d speed %.1f", b.Type, b.Mode, b.Def.Speed)
	}
}
