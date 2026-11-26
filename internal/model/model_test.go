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

func TestStorePublishesWithoutBlockingOnSlowSubscribers(t *testing.T) {
	s := NewStore(nil)
	stream, cancel := s.Subscribe()
	defer cancel()
	for i := 0; i < 40; i++ {
		s.Add(Connection{ID: string(rune(i))})
	}
	count := 0
	for {
		select {
		case <-stream:
			count++
		default:
			if count != 32 {
				t.Fatalf("got %d buffered events, want bounded 32", count)
			}
			return
		}
	}
}
