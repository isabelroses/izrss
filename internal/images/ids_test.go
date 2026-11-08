package images

import (
	"strconv"
	"testing"
)

func TestIDs(t *testing.T) {
	ids := NewIDs()
	a, ev := ids.Acquire("a")
	if a < 1 || a > 255 || ev != 0 {
		t.Fatalf("first id = %d, evicted %d", a, ev)
	}
	if again, _ := ids.Acquire("a"); again != a {
		t.Errorf("same key should keep its id: %d vs %d", again, a)
	}
	for i := range 254 {
		if _, ev := ids.Acquire("k" + strconv.Itoa(i)); ev != 0 {
			t.Fatalf("no eviction expected while filling, got %d", ev)
		}
	}
	// "a" is now the least recently used; refresh it so k0 becomes LRU.
	ids.Acquire("a")
	k0, _ := ids.Acquire("k0")
	ids.Acquire("a")
	for i := 1; i < 254; i++ {
		ids.Acquire("k" + strconv.Itoa(i))
	}
	id, evicted := ids.Acquire("new")
	if evicted == 0 || evicted != k0 || id != k0 {
		t.Errorf("expected k0's id %d to be evicted and reused, got id %d evicted %d", k0, id, evicted)
	}
	if got, _ := ids.Acquire("a"); got != a {
		t.Errorf("recently used key lost its id")
	}
}
