package stage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/core/internal/stage"
	"github.com/pobear/pobear/services/core/internal/stage/extractors"
)

type fakeExtractor struct {
	results []extractors.Result
	err     error
	name    string
	order   *[]string

	gotCfgs []jobspec.Stage
	calls   int
}

func (f *fakeExtractor) Run(_ context.Context, _ []byte, cfg jobspec.Stage) ([]extractors.Result, error) {
	f.gotCfgs = append(f.gotCfgs, cfg)
	f.calls++
	if f.order != nil {
		*f.order = append(*f.order, f.name)
	}
	return f.results, f.err
}

func TestStagerRunsEachConfiguredStageInOrder(t *testing.T) {
	order := &[]string{}
	raw := &fakeExtractor{results: []extractors.Result{{Key: "raw", Value: "body"}}, name: "raw", order: order}
	md := &fakeExtractor{results: []extractors.Result{{Key: "markdown", Value: "# body"}}, name: "markdown", order: order}

	s := stage.NewStager(map[string]stage.Extractor{"raw": raw, "markdown": md})
	cfgs := []jobspec.Stage{{Use: "raw"}, {Use: "markdown"}}
	data, err := s.Run(context.Background(), []byte("body"), cfgs)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data["raw"] != "body" || data["markdown"] != "# body" {
		t.Fatalf("data = %+v, want both raw and markdown keys populated", data)
	}
	if raw.calls != 1 || md.calls != 1 {
		t.Fatalf("expected each stage called exactly once, got raw=%d markdown=%d", raw.calls, md.calls)
	}
	want := []string{"raw", "markdown"}
	if len(*order) != len(want) || (*order)[0] != want[0] || (*order)[1] != want[1] {
		t.Fatalf("dispatch order = %v, want %v", *order, want)
	}
}

func TestStagerMergesMultipleResultsFromOneStage(t *testing.T) {
	css := &fakeExtractor{results: []extractors.Result{
		{Key: "title", Value: "Hello"},
		{Key: "price", Value: "9.99"},
	}}

	s := stage.NewStager(map[string]stage.Extractor{"css": css})
	data, err := s.Run(context.Background(), []byte("<h1>Hello</h1>"), []jobspec.Stage{{Use: "css"}})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data["title"] != "Hello" || data["price"] != "9.99" {
		t.Fatalf("expected both results merged into data, got %+v", data)
	}
}

func TestStagerNoConfiguredStages(t *testing.T) {
	s := stage.NewStager(map[string]stage.Extractor{})
	data, err := s.Run(context.Background(), []byte("body"), nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected no data, got %+v", data)
	}
}

func TestStagerPassesEachStageItsOwnConfig(t *testing.T) {
	css := &fakeExtractor{results: []extractors.Result{{Key: "title", Value: "Hello"}}}

	s := stage.NewStager(map[string]stage.Extractor{"css": css})
	cfgs := []jobspec.Stage{
		{Use: "css", Fields: map[string]string{"title": "h1"}},
		{Use: "css", Fields: map[string]string{"price": ".price"}},
	}
	_, err := s.Run(context.Background(), []byte("<h1>Hello</h1>"), cfgs)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(css.gotCfgs) != 2 {
		t.Fatalf("expected 2 calls recorded, got %d", len(css.gotCfgs))
	}
	if css.gotCfgs[0].Fields["title"] != "h1" {
		t.Fatalf("expected the first call's own cfg, got %+v", css.gotCfgs[0])
	}
	if css.gotCfgs[1].Fields["price"] != ".price" {
		t.Fatalf("expected the second call's own cfg, got %+v", css.gotCfgs[1])
	}
}

func TestStagerUnknownStageName(t *testing.T) {
	raw := &fakeExtractor{results: []extractors.Result{{Key: "raw", Value: "body"}}}
	s := stage.NewStager(map[string]stage.Extractor{"raw": raw})
	cfgs := []jobspec.Stage{{Use: "raw"}, {Use: "missing"}, {Use: "raw"}}
	_, err := s.Run(context.Background(), []byte("body"), cfgs)

	if !errors.Is(err, stage.ErrUnknownExtractor) {
		t.Fatalf("expected ErrUnknownExtractor, got %v", err)
	}
	if raw.calls != 1 {
		t.Fatalf("expected the first raw stage to run once and the third (after the unknown one) never to run, got %d calls", raw.calls)
	}
}

func TestStagerStopsAtFirstStageError(t *testing.T) {
	wantErr := errors.New("boom")
	raw := &fakeExtractor{err: wantErr}
	md := &fakeExtractor{results: []extractors.Result{{Key: "markdown", Value: "should never run"}}}

	s := stage.NewStager(map[string]stage.Extractor{"raw": raw, "markdown": md})
	cfgs := []jobspec.Stage{{Use: "raw"}, {Use: "markdown"}}
	_, err := s.Run(context.Background(), []byte("body"), cfgs)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the first stage's error, got %v", err)
	}
	if errors.Is(err, stage.ErrUnknownExtractor) {
		t.Fatalf("expected a stage error distinct from ErrUnknownExtractor, got %v", err)
	}
	if md.calls != 0 {
		t.Fatalf("expected the second stage never called after the first errored, got %d calls", md.calls)
	}
}
