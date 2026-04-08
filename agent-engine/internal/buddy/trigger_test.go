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

func TestFindBuddyTriggerPositions_WordBoundary(t *testing.T) {
	// "/buddyX" should NOT match (no word boundary)
	ranges := FindBuddyTriggerPositions("/buddyX")
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges for /buddyX, got %d", len(ranges))
	}

	// "/buddy_foo" should NOT match
	ranges = FindBuddyTriggerPositions("/buddy_foo")
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges for /buddy_foo, got %d", len(ranges))
	}

	// "/buddy123" should NOT match
	ranges = FindBuddyTriggerPositions("/buddy123")
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges for /buddy123, got %d", len(ranges))
	}

	// "/buddy " should match
	ranges = FindBuddyTriggerPositions("/buddy pet")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range for '/buddy pet', got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 6 {
		t.Errorf("range: %+v", ranges[0])
	}

	// "/buddy" at end of string should match
	ranges = FindBuddyTriggerPositions("try /buddy")
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range for 'try /buddy', got %d", len(ranges))
	}
}
