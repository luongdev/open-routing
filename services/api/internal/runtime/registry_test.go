package runtime

import "testing"

// TestDefaultRegistry_CoversV02Subset proves the registry contains exactly the
// locked v0.2 node subset, in order, each with a populated descriptor — so a
// node can't be half-added (kind without descriptor, or descriptor without
// registration).
func TestDefaultRegistry_CoversV02Subset(t *testing.T) {
	r := DefaultRegistry()
	got := r.Kinds()
	if len(got) != len(V02NodeKinds) {
		t.Fatalf("registry has %d kinds, want %d", len(got), len(V02NodeKinds))
	}
	for i, want := range V02NodeKinds {
		if got[i] != want {
			t.Fatalf("kind[%d] = %q, want %q (order must match V02NodeKinds)", i, got[i], want)
		}
		n, ok := r.Lookup(want)
		if !ok {
			t.Fatalf("Lookup(%q) failed", want)
		}
		d := n.Descriptor()
		if d.Kind != want || d.Title == "" || d.Category == "" {
			t.Fatalf("node %q has incomplete descriptor: %+v", want, d)
		}
	}
}

// TestRegistry_DuplicatePanics guards the seam: registering a kind twice is a
// startup-time programming error, not a silent last-write-wins.
func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r := NewRegistry()
	r.Register(endNode{base(NodeEnd)})
	r.Register(endNode{base(NodeEnd)})
}
