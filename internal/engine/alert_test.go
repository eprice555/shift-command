package engine

import (
	"testing"

	"github.com/eprice555/shift-command/internal/geom"
)

func TestUnderAttackAlert(t *testing.T) {
	m := NewMatch(false)
	if m.ai == nil {
		t.Fatal("skirmish has no AI opponent")
	}
	if NewMatch(true).ai != nil {
		t.Fatal("sandbox should have no AI opponent")
	}
	far := m.world.Map.Starts[1-m.player]
	vis := m.cam.Visible()
	m.underAttack(vis.Min.Add(vis.Max).Scale(0.5))
	if m.hasAlert {
		t.Fatal("on-screen attack raised an alert")
	}
	m.underAttack(far)
	if !m.hasAlert || m.alert != far || m.msg == "" {
		t.Fatal("off-screen attack raised no alert")
	}
	m.updates += alertCooldown - 1
	m.underAttack(far.Add(geom.V(3, 0)))
	if m.alert != far {
		t.Fatal("alert repeated within the cooldown")
	}
	m.updates++
	m.underAttack(far.Add(geom.V(3, 0)))
	if m.alert == far {
		t.Fatal("alert suppressed after the cooldown")
	}
}
