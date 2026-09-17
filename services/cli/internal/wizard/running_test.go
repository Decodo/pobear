package wizard

import (
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunProgressPercentTracksCompletedOverTotal(t *testing.T) {
	p := newRunProgress(4)
	if got := p.percent(); got != 0 {
		t.Fatalf("percent() before anything completes = %v, want 0", got)
	}
	p.completed.Store(2)
	if got := p.percent(); got != 0.5 {
		t.Fatalf("percent() at 2/4 = %v, want 0.5", got)
	}
	p.completed.Store(4)
	if got := p.percent(); got != 1 {
		t.Fatalf("percent() at 4/4 = %v, want 1", got)
	}
}

func TestRunProgressPercentNeverExceedsOne(t *testing.T) {
	p := newRunProgress(2)
	p.completed.Store(5)
	if got := p.percent(); got != 1 {
		t.Fatalf("percent() = %v, want clamped to 1", got)
	}
}

func TestRunProgressPercentOnNilIsZero(t *testing.T) {
	var p *runProgress
	if got := p.percent(); got != 0 {
		t.Fatalf("percent() on nil progress = %v, want 0", got)
	}
}

func TestCountingWriterIncrementsOncePerWriteCall(t *testing.T) {
	var buf strings.Builder
	var counter atomic.Int64
	w := &countingWriter{out: &buf, counter: &counter}

	for i := 0; i < 3; i++ {
		if _, err := w.Write([]byte("line\n")); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if got := counter.Load(); got != 3 {
		t.Fatalf("counter after 3 writes = %d, want 3", got)
	}
	if buf.String() != "line\nline\nline\n" {
		t.Fatalf("buf = %q, want the writes to still reach the wrapped writer", buf.String())
	}
}

func TestCountingWriterIsSafeForConcurrentWrites(t *testing.T) {
	var counter atomic.Int64
	w := &countingWriter{out: io.Discard, counter: &counter}

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.Write([]byte("x"))
		}()
	}
	wg.Wait()

	if got := counter.Load(); got != n {
		t.Fatalf("counter after %d concurrent writes = %d, want %d", n, got, n)
	}
}

func TestRunningScreenTransitionsToResultsOnFinishWithRows(t *testing.T) {
	form := &scrapeForm{}
	s := newRunningScreen(form, newRunProgress(1))

	rows := []resultRow{{url: "https://a.test", status: "✓ 200"}}
	_, cmd := s.Update(runFinishedMsg{status: "✓ fetched 1, errors 0, skipped 0", rows: rows})
	if form.status != "✓ fetched 1, errors 0, skipped 0" {
		t.Fatalf("form.status = %q, want the finished status to be set on the underlying form", form.status)
	}
	if cmd == nil {
		t.Fatal("expected a command to replace the top screen, got nil")
	}
	msg := cmd()
	replace, ok := msg.(replaceTopMsg)
	if !ok {
		t.Fatalf("expected a replaceTopMsg, got %T", msg)
	}
	if _, ok := replace.screen.(*resultsScreen); !ok {
		t.Fatalf("expected the replacement screen to be *resultsScreen, got %T", replace.screen)
	}
}

func TestRunningScreenPopsBackToFormOnFinishWithNoRows(t *testing.T) {
	form := &scrapeForm{}
	s := newRunningScreen(form, newRunProgress(1))

	_, cmd := s.Update(runFinishedMsg{status: "✗ boom", rows: nil})
	if form.status != "✗ boom" {
		t.Fatalf("form.status = %q, want the failure status set on the form", form.status)
	}
	if cmd == nil {
		t.Fatal("expected a pop command, got nil")
	}
	if _, ok := cmd().(popScreenMsg); !ok {
		t.Fatalf("expected a popScreenMsg, got %T", cmd())
	}
}

func TestRunningScreenTicksKeepFiring(t *testing.T) {
	s := newRunningScreen(&scrapeForm{}, newRunProgress(1))
	_, cmd := s.Update(runTickMsg{})
	if cmd == nil {
		t.Fatal("expected the tick to reschedule itself, got nil")
	}
}

func TestRenderProgressBarClampsAndScales(t *testing.T) {
	if got := renderProgressBar(0); !strings.Contains(got, strings.Repeat("░", progressBarWidth)) {
		t.Fatalf("renderProgressBar(0) = %q, want an all-empty bar", got)
	}
	if got := renderProgressBar(1); !strings.Contains(got, strings.Repeat("█", progressBarWidth)) {
		t.Fatalf("renderProgressBar(1) = %q, want an all-filled bar", got)
	}
	if got := renderProgressBar(2); got != renderProgressBar(1) {
		t.Fatalf("renderProgressBar(2) = %q, want it clamped the same as renderProgressBar(1) = %q", got, renderProgressBar(1))
	}
}

func TestRunningScreenViewShowsPercentAndCounts(t *testing.T) {
	p := newRunProgress(4)
	p.completed.Store(1)
	s := newRunningScreen(&scrapeForm{}, p)
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	view := s.View()
	if !strings.Contains(view, "25%") || !strings.Contains(view, "(1/4)") {
		t.Fatalf("View() = %q, want it to show 25%% and (1/4)", view)
	}
}

func TestRunningScreenShowsBearOnTopWhenThereIsRoom(t *testing.T) {
	s := newRunningScreen(&scrapeForm{}, newRunProgress(1))
	s.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	if !strings.Contains(s.View(), "(ᴥ)") {
		t.Fatal("expected the bear to be visible on a tall terminal")
	}
}

func TestRunningScreenNeverPlacesBearOnTheSide(t *testing.T) {
	s := newRunningScreen(&scrapeForm{}, newRunProgress(1))
	s.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	lines := strings.Split(s.View(), "\n")
	bearLine, boxLine := -1, -1
	for i, l := range lines {
		if bearLine == -1 && strings.Contains(l, "(ᴥ)") {
			bearLine = i
		}
		if strings.Contains(l, "┌─ Running") {
			boxLine = i
		}
	}
	if bearLine == -1 || boxLine == -1 {
		t.Fatalf("expected both the bear and the box border in the view, got bearLine=%d boxLine=%d", bearLine, boxLine)
	}
	if bearLine >= boxLine {
		t.Fatalf("bear appeared at line %d, box border at line %d -- want the bear strictly above the box (top placement only, never beside it)", bearLine, boxLine)
	}
}
