package privilege

import "testing"

const testNetCaps = uint32(1<<12 | 1<<13) // cap_net_admin, cap_net_raw

func TestRaiseAmbientNetCapsNoPanic(t *testing.T) {
	RaiseAmbientNetCaps()
}

func TestCapGetSucceeds(t *testing.T) {
	sets, err := capGet()
	if err != nil {
		t.Fatalf("capget: %v", err)
	}
	for i := range sets {
		// inheritable must not carry net caps the permitted set lacks
		if sets[i].inheritable&^sets[i].permitted&testNetCaps != 0 {
			t.Fatalf("word %d: inheritable has net caps not in permitted: %+v", i, sets[i])
		}
	}
}
