package buddy

import "testing"

func TestFindBuddyTriggerPositions_Single(t *testing.T) {
	ranges := FindBuddyTriggerPositions("/buddy")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 6 {
		t.Errorf("range: %+v", ranges[0])
	}
}

func TestFindBuddyTriggerPositions_Multiple(t *testing.T) {
	ranges := FindBuddyTriggerPositions("try /buddy and /buddy pet")
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}
	if ranges[0].Start != 4 || ranges[0].End != 10 {
		t.Errorf("first range: %+v", ranges[0])
	}
	if ranges[1].Start != 15 || ranges[1].End != 21 {
		t.Errorf("second range: %+v", ranges[1])
	}
}

func TestFindBuddyTriggerPositions_None(t *testing.T) {
	ranges := FindBuddyTriggerPositions("hello world")
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges, got %d", len(ranges))
	}
}

func TestFindBuddyTriggerPositions_Empty(t *testing.T) {
	ranges := FindBuddyTriggerPositions("")
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges, got %d", len(ranges))
	}
}
