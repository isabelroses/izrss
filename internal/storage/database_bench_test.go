package storage

import (
	"fmt"
	"testing"
)

func benchDB(b *testing.B) *DB {
	b.Helper()
	db, err := New(b.TempDir() + "/bench.db")
	if err != nil {
		b.Fatalf("opening db: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })
	return db
}

func makeStatuses(n int) []PostReadStatus {
	statuses := make([]PostReadStatus, n)
	for i := range statuses {
		statuses[i] = PostReadStatus{
			UUID:    fmt.Sprintf("uuid-%d", i),
			FeedURL: fmt.Sprintf("https://example.com/feed-%d", i%20),
			Read:    i%2 == 0,
		}
	}
	return statuses
}

// BenchmarkWriteAll measures the bulk write, whose cost scales with the library.
func BenchmarkWriteAll(b *testing.B) {
	for _, n := range []int{100, 800, 2000} {
		b.Run(fmt.Sprintf("posts=%d", n), func(b *testing.B) {
			db := benchDB(b)
			statuses := makeStatuses(n)
			b.ReportAllocs()
			for b.Loop() {
				if err := db.SavePostReadStatuses(statuses); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWriteOne persists a single post; cost is flat regardless of library size.
func BenchmarkWriteOne(b *testing.B) {
	for _, n := range []int{100, 800, 2000} {
		b.Run(fmt.Sprintf("posts=%d", n), func(b *testing.B) {
			db := benchDB(b)
			statuses := makeStatuses(n)
			// Seed so the single write is an update, matching real use.
			if err := db.SavePostReadStatuses(statuses); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				s := statuses[i%n]
				if err := db.SavePostReadStatus(s.UUID, s.FeedURL, !s.Read); err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	}
}
