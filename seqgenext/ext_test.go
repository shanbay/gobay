package seqgenext

import (
	"fmt"
	"testing"

	"github.com/go-redis/redis"
)

func TestSequenceGenerator(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer client.Close()
	g := SequenceGeneratorExt{
		redisClient:  client,
		SequenceBase: 0,
		SequenceKey:  "test_key",
	}
	_, err := g.getSequence(MAX_SEQUENCE + 1)
	expectedErr := fmt.Errorf("step should not less than 1 or greater than MAX_STEP(%d)", MAX_STEP)
	if err == nil || err.Error() != expectedErr.Error() {
		t.Errorf("err `%s` is not expected, expected `%s`", err, expectedErr)
	}

	count := 10
	sequencesSet := make(map[uint64]struct{}, 10)
	for i := 0; i < count; i++ {
		sequence, err := g.GetSequence()
		if err != nil {
			t.Fatal(err)
		}
		sequencesSet[sequence] = struct{}{}
	}
	if len(sequencesSet) != count {
		t.Fatalf("GetSequence count %d, expected %d", len(sequencesSet), count)
	}

	sequences := g.GetSequences(uint64(count), 3)
	for sequences.HasNext() {
		sequence, err := sequences.Next()
		if err != nil {
			t.Fatal(err)
		}
		sequencesSet[sequence] = struct{}{}
	}
	if len(sequencesSet) != count*2 {
		t.Fatalf("GetSequences count %d, expected %d", len(sequencesSet)-count, count)
	}

	sequence, err := sequences.Next()
	if err != nil {
		t.Fatal(err)
	}
	sequencesSet[sequence] = struct{}{}
	if len(sequencesSet) != count*2 {
		t.Fatalf("GetSequences count %d, expected %d", len(sequencesSet)-count, count)
	}
}

func BenchmarkSequenceGenerator_GetSequence(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer client.Close()
	g := SequenceGeneratorExt{
		redisClient:  client,
		SequenceBase: 0,
		SequenceKey:  "test_key",
	}
	for i := 0; i < b.N; i++ {
		if _, err := g.GetSequence(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSequenceGenerator_GetSequences_100(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer client.Close()
	g := SequenceGeneratorExt{
		redisClient:  client,
		SequenceBase: 0,
		SequenceKey:  "test_key",
	}
	count := 100
	for i := 0; i < b.N; i++ {
		sequences := g.GetSequences(uint64(count), 100)
		for sequences.HasNext() {
			if _, err := sequences.Next(); err != nil {
				b.Fatal(err)
			}
		}
	}
}
