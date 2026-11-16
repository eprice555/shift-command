package engine

import "testing"

func TestFixedStep20Of60(t *testing.T) {
	f := NewFixedStep(20, 60)
	total := 0
	for i := 1; i <= 60; i++ {
		n := f.Advance()
		total += n
		want := 0
		if i%3 == 0 {
			want = 1
		}
		if n != want {
			t.Fatalf("update %d: got %d ticks, want %d", i, n, want)
		}
		if a := f.Alpha(); a < 0 || a >= 1 {
			t.Fatalf("alpha %v out of [0,1)", a)
		}
	}
	if total != 20 {
		t.Errorf("total ticks = %d, want 20", total)
	}
}

func TestFixedStepFasterSimThanUpdates(t *testing.T) {
	f := NewFixedStep(60, 20)
	if n := f.Advance(); n != 3 {
		t.Errorf("got %d ticks, want 3", n)
	}
}

func TestFixedStepUnevenRatio(t *testing.T) {
	f := NewFixedStep(20, 144)
	total := 0
	for range 144 * 5 {
		total += f.Advance()
	}
	if total != 100 {
		t.Errorf("total ticks = %d, want 100", total)
	}
}
