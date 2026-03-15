package carrier

import "testing"

func TestProfileRegistry_Register(t *testing.T) {
	reg := NewProfileRegistry()
	reg.Register(ExecutionProfile{ID: "prof_1", CarrierID: "carrier_a", Name: "GPU Inference", Agent: "gpt-4"})

	p, ok := reg.Get("prof_1")
	if !ok {
		t.Fatal("expected profile")
	}
	if p.Agent != "gpt-4" {
		t.Errorf("agent = %s", p.Agent)
	}
}

func TestProfileRegistry_List(t *testing.T) {
	reg := NewProfileRegistry()
	reg.Register(ExecutionProfile{ID: "p1", CarrierID: "carrier_a"})
	reg.Register(ExecutionProfile{ID: "p2", CarrierID: "carrier_b"})

	all := reg.List("")
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}

	filtered := reg.List("carrier_a")
	if len(filtered) != 1 {
		t.Errorf("expected 1, got %d", len(filtered))
	}
}

func TestProfileRegistry_Delete(t *testing.T) {
	reg := NewProfileRegistry()
	reg.Register(ExecutionProfile{ID: "p1", CarrierID: "carrier_a"})
	reg.Delete("p1")

	_, ok := reg.Get("p1")
	if ok {
		t.Error("expected deleted")
	}
}

func TestProfileRegistry_GetMissing(t *testing.T) {
	reg := NewProfileRegistry()
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}
