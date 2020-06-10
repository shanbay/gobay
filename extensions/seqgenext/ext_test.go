package seqgenext

import (
	"context"
	"fmt"
	"testing"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/redisext"
)

var (
	ctx    = context.Background()
	app, _ = gobay.CreateApp("../../testdata/", "testing", map[gobay.Key]gobay.Extension{
		"redis": &redisext.RedisExt{
			NS: "redis_",
		},
		"seqgen": &SequenceGeneratorExt{
			NS:           "seqgen_",
			RedisExtName: "redis",
			SequenceBase: 0,
			SequenceKey:  "test_key",
		},
	})
)

func TestSequenceGenerator(t *testing.T) {
	g := app.Get("seqgen").Object().(*SequenceGeneratorExt)
	_, err := g.getSequence(ctx, maxSequence+1)
	expectedErr := fmt.Errorf("step should not less than 1 or greater than MAX_STEP(%d)", maxStep)
	if err == nil || err.Error() != expectedErr.Error() {
		t.Errorf("err `%s` is not expected, expected `%s`", err, expectedErr)
	}

	count := 10
	sequencesSet := make(map[uint64]struct{}, 10)
	for i := 0; i < count; i++ {
		sequence, err := g.GetSequence(ctx)
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
		sequence, err := sequences.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		sequencesSet[sequence] = struct{}{}
	}
	if len(sequencesSet) != count*2 {
		t.Fatalf("GetSequences count %d, expected %d", len(sequencesSet)-count, count)
	}

	sequence, err := sequences.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sequencesSet[sequence] = struct{}{}
	if len(sequencesSet) != count*2 {
		t.Fatalf("GetSequences count %d, expected %d", len(sequencesSet)-count, count)
	}
}

func BenchmarkSequenceGenerator_GetSequence(b *testing.B) {
	g := app.Get("seqgen").Object().(*SequenceGeneratorExt)
	for i := 0; i < b.N; i++ {
		if _, err := g.GetSequence(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSequenceGenerator_GetSequences_100(b *testing.B) {
	g := app.Get("seqgen").Object().(*SequenceGeneratorExt)
	count := 100
	for i := 0; i < b.N; i++ {
		sequences := g.GetSequences(uint64(count), 100)
		for sequences.HasNext() {
			if _, err := sequences.Next(ctx); err != nil {
				b.Fatal(err)
			}
		}
	}
}
