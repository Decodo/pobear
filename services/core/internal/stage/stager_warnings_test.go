package stage_test

import (
	"context"
	"testing"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/stage"
	"github.com/pobear/pobear/services/core/internal/stage/extractors"
)

func TestStagerSecondWriteWinsOnKeyCollision(t *testing.T) {
	first := &fakeExtractor{results: []extractors.Result{{Key: "title", Value: "first"}}}
	second := &fakeExtractor{results: []extractors.Result{{Key: "title", Value: "second"}}}

	s := stage.NewStager(map[string]stage.Extractor{"first": first, "second": second})
	cfgs := []jobspec.Stage{{Use: "first"}, {Use: "second"}}
	data, warnings, err := s.Run(context.Background(), []byte("body"), cfgs)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data["title"] != "second" {
		t.Fatalf("data[title] = %v, want %q (the second write)", data["title"], "second")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 warning, got %d: %v", len(warnings), warnings)
	}
	want := `stages[1] (second) overwrites data["title"] previously written by stages[0] (first)`
	if warnings[0] != want {
		t.Fatalf("warning = %q, want %q", warnings[0], want)
	}
}

func TestStagerThreeWayKeyCollision(t *testing.T) {
	first := &fakeExtractor{results: []extractors.Result{{Key: "title", Value: "first"}}}
	second := &fakeExtractor{results: []extractors.Result{{Key: "title", Value: "second"}}}
	third := &fakeExtractor{results: []extractors.Result{{Key: "title", Value: "third"}}}

	s := stage.NewStager(map[string]stage.Extractor{"first": first, "second": second, "third": third})
	cfgs := []jobspec.Stage{{Use: "first"}, {Use: "second"}, {Use: "third"}}
	data, warnings, err := s.Run(context.Background(), []byte("body"), cfgs)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data["title"] != "third" {
		t.Fatalf("data[title] = %v, want %q (the last write)", data["title"], "third")
	}
	if len(warnings) != 2 {
		t.Fatalf("expected exactly 2 warnings, got %d: %v", len(warnings), warnings)
	}
	want0 := `stages[1] (second) overwrites data["title"] previously written by stages[0] (first)`
	want1 := `stages[2] (third) overwrites data["title"] previously written by stages[1] (second)`
	if warnings[0] != want0 {
		t.Fatalf("warnings[0] = %q, want %q", warnings[0], want0)
	}
	if warnings[1] != want1 {
		t.Fatalf("warnings[1] = %q, want %q", warnings[1], want1)
	}
}

func TestStagerNilValuedResult(t *testing.T) {
	solo := &fakeExtractor{results: []extractors.Result{{Key: "price", Value: nil}}}
	s := stage.NewStager(map[string]stage.Extractor{"solo": solo})
	data, warnings, err := s.Run(context.Background(), []byte("body"), []jobspec.Stage{{Use: "solo"}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if v, ok := data["price"]; !ok || v != nil {
		t.Fatalf("data[price] = %v (present=%v), want present with a nil value", v, ok)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	first := &fakeExtractor{results: []extractors.Result{{Key: "price", Value: nil}}}
	second := &fakeExtractor{results: []extractors.Result{{Key: "price", Value: "9.99"}}}
	s2 := stage.NewStager(map[string]stage.Extractor{"first": first, "second": second})
	cfgs := []jobspec.Stage{{Use: "first"}, {Use: "second"}}
	data2, warnings2, err := s2.Run(context.Background(), []byte("body"), cfgs)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data2["price"] != "9.99" {
		t.Fatalf("data[price] = %v, want %q (the later write)", data2["price"], "9.99")
	}
	if len(warnings2) != 1 {
		t.Fatalf("expected exactly 1 warning, got %d: %v", len(warnings2), warnings2)
	}
	want := `stages[1] (second) overwrites data["price"] previously written by stages[0] (first)`
	if warnings2[0] != want {
		t.Fatalf("warning = %q, want %q", warnings2[0], want)
	}
}

func TestStagerNoWarningWhenKeysDontCollide(t *testing.T) {
	raw := &fakeExtractor{results: []extractors.Result{{Key: "raw", Value: "body"}}}
	md := &fakeExtractor{results: []extractors.Result{{Key: "markdown", Value: "# body"}}}

	s := stage.NewStager(map[string]stage.Extractor{"raw": raw, "markdown": md})
	cfgs := []jobspec.Stage{{Use: "raw"}, {Use: "markdown"}}
	_, warnings, err := s.Run(context.Background(), []byte("body"), cfgs)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}
