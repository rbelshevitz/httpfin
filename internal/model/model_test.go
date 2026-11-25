package model

import "testing"

func TestStoreKeepsNewestFirstAndBounded(t *testing.T) {
	s := NewStore(nil)
	for i := 0; i < 5003; i++ {
		s.Add(Connection{ID: string(rune(i))})
	}
	items := s.List()
	if len(items) != defaultCapacity {
		t.Fatalf("got %d items, want %d", len(items), defaultCapacity)
	}
	if items[0].ID != string(rune(5002)) || items[len(items)-1].ID != string(rune(3)) {
		t.Fatalf("unexpected ring order: first=%q last=%q", items[0].ID, items[len(items)-1].ID)
	}
}
