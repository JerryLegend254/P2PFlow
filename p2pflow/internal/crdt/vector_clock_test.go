package crdt

import (
	"testing"
)

func TestVectorClock_Increment(t *testing.T) {
	vc := NewVectorClock()

	vc.Increment("node1")
	if vc.Get("node1") != 1 {
		t.Errorf("Expected 1, got %d", vc.Get("node1"))
	}

	vc.Increment("node1")
	if vc.Get("node1") != 2 {
		t.Errorf("Expected 2, got %d", vc.Get("node1"))
	}
}

func TestVectorClock_Merge(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 5)
	vc1.Set("node2", 3)

	vc2 := NewVectorClock()
	vc2.Set("node1", 3)
	vc2.Set("node2", 7)
	vc2.Set("node3", 2)

	vc1.Merge(vc2)

	if vc1.Get("node1") != 5 {
		t.Errorf("Expected 5, got %d", vc1.Get("node1"))
	}
	if vc1.Get("node2") != 7 {
		t.Errorf("Expected 7, got %d", vc1.Get("node2"))
	}
	if vc1.Get("node3") != 2 {
		t.Errorf("Expected 2, got %d", vc1.Get("node3"))
	}
}

func TestVectorClock_Compare_Equal(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 5)
	vc1.Set("node2", 3)

	vc2 := NewVectorClock()
	vc2.Set("node1", 5)
	vc2.Set("node2", 3)

	result := vc1.Compare(vc2)
	if result != Equal {
		t.Errorf("Expected Equal, got %v", result)
	}
}

func TestVectorClock_Compare_Before(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 3)
	vc1.Set("node2", 2)

	vc2 := NewVectorClock()
	vc2.Set("node1", 5)
	vc2.Set("node2", 4)

	result := vc1.Compare(vc2)
	if result != Before {
		t.Errorf("Expected Before, got %v", result)
	}
}

func TestVectorClock_Compare_After(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 5)
	vc1.Set("node2", 4)

	vc2 := NewVectorClock()
	vc2.Set("node1", 3)
	vc2.Set("node2", 2)

	result := vc1.Compare(vc2)
	if result != After {
		t.Errorf("Expected After, got %v", result)
	}
}

func TestVectorClock_Compare_Concurrent(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 5)
	vc1.Set("node2", 2)

	vc2 := NewVectorClock()
	vc2.Set("node1", 3)
	vc2.Set("node2", 4)

	result := vc1.Compare(vc2)
	if result != Concurrent {
		t.Errorf("Expected Concurrent, got %v", result)
	}
}

func TestVectorClock_Copy(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 5)
	vc1.Set("node2", 3)

	vc2 := vc1.Copy()

	// Modify original
	vc1.Set("node1", 10)

	// Copy should not be affected
	if vc2.Get("node1") != 5 {
		t.Errorf("Expected 5, got %d", vc2.Get("node1"))
	}
}

func TestVectorClock_HappenedBefore(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 3)
	vc1.Set("node2", 2)

	vc2 := NewVectorClock()
	vc2.Set("node1", 5)
	vc2.Set("node2", 4)

	if !vc1.HappenedBefore(vc2) {
		t.Error("Expected vc1 to happen before vc2")
	}

	if vc2.HappenedBefore(vc1) {
		t.Error("Expected vc2 not to happen before vc1")
	}
}

func TestVectorClock_IsConcurrent(t *testing.T) {
	vc1 := NewVectorClock()
	vc1.Set("node1", 5)
	vc1.Set("node2", 2)

	vc2 := NewVectorClock()
	vc2.Set("node1", 3)
	vc2.Set("node2", 4)

	if !vc1.IsConcurrent(vc2) {
		t.Error("Expected vc1 and vc2 to be concurrent")
	}
}
