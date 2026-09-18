package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeScreen struct {
	width, height int
	view          string
}

func (f fakeScreen) Init() tea.Cmd { return nil }

func (f fakeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		f.width, f.height = wsm.Width, wsm.Height
	}
	return f, nil
}

func (f fakeScreen) View() string { return f.view }

type initTrackingScreen struct {
	fakeScreen
	initCalled *bool
}

func (f initTrackingScreen) Init() tea.Cmd {
	*f.initCalled = true
	return func() tea.Msg { return "init-cmd-ran" }
}

func newInitTrackingScreen(view string) (tea.Model, *bool) {
	called := false
	return initTrackingScreen{fakeScreen: fakeScreen{view: view}, initCalled: &called}, &called
}

func TestAppCtrlCAlwaysQuits(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected ctrl+c to return a quit command, got nil")
	}
}

func TestAppPushMakesTopScreenVisible(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	updated, _ := a.Update(pushScreenMsg{screen: fakeScreen{view: "pushed"}})
	got := updated.(app)
	if got.View() != "pushed" {
		t.Fatalf("View() = %q, want %q", got.View(), "pushed")
	}
}

func TestAppPopReturnsToPreviousScreen(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	updated, _ := a.Update(pushScreenMsg{screen: fakeScreen{view: "pushed"}})
	a = updated.(app)

	updated, _ = a.Update(popScreenMsg{})
	got := updated.(app)
	if got.View() != "root" {
		t.Fatalf("View() after pop = %q, want %q", got.View(), "root")
	}
}

func TestAppPopOnRootIsANoOp(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	updated, _ := a.Update(popScreenMsg{})
	got := updated.(app)
	if got.View() != "root" {
		t.Fatalf("popping the root screen should be a no-op, got View() = %q", got.View())
	}
}

func TestAppPushCallsInitOnTheNewScreen(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	screen, called := newInitTrackingScreen("pushed")

	_, cmd := a.Update(pushScreenMsg{screen: screen})
	if !*called {
		t.Fatal("expected pushScreen to call Init() on the pushed screen")
	}
	if cmd == nil {
		t.Fatal("expected the pushed screen's Init() command to be returned (batched), got nil")
	}
}

func TestAppReplaceTopCallsInitOnTheNewScreen(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	updated, _ := a.Update(pushScreenMsg{screen: fakeScreen{view: "loading"}})
	a = updated.(app)

	screen, called := newInitTrackingScreen("results")
	_, cmd := a.Update(replaceTopMsg{screen: screen})
	if !*called {
		t.Fatal("expected replaceTop to call Init() on the replacement screen")
	}
	if cmd == nil {
		t.Fatal("expected the replacement screen's Init() command to be returned (batched), got nil")
	}
}

func TestAppReplaceTopSwapsAtomicallyAndKeepsDeeperScreens(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	updated, _ := a.Update(pushScreenMsg{screen: fakeScreen{view: "loading"}})
	a = updated.(app)

	updated, _ = a.Update(replaceTopMsg{screen: fakeScreen{view: "results"}})
	a = updated.(app)

	if got := a.View(); got != "results" {
		t.Fatalf("View() after replaceTop = %q, want %q", got, "results")
	}
	if len(a.stack) != 2 {
		t.Fatalf("stack depth = %d, want 2 (root still underneath)", len(a.stack))
	}

	updated, _ = a.Update(popScreenMsg{})
	a = updated.(app)
	if got := a.View(); got != "root" {
		t.Fatalf("View() after popping the replaced screen = %q, want %q (root untouched)", got, "root")
	}
}

func TestAppPushGivesTheNewScreenTheCurrentWindowSize(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	updated, _ := a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a = updated.(app)

	updated, _ = a.Update(pushScreenMsg{screen: fakeScreen{view: "pushed"}})
	a = updated.(app)

	top := a.top().(fakeScreen)
	if top.width != 80 || top.height != 24 {
		t.Fatalf("pushed screen got size %dx%d, want 80x24", top.width, top.height)
	}
}

func TestAppWindowSizePropagatesToEveryScreenInTheStack(t *testing.T) {
	a := newApp(fakeScreen{view: "root"})
	updated, _ := a.Update(pushScreenMsg{screen: fakeScreen{view: "pushed"}})
	a = updated.(app)

	updated, _ = a.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	a = updated.(app)

	for i, screen := range a.stack {
		fs := screen.(fakeScreen)
		if fs.width != 100 || fs.height != 40 {
			t.Errorf("stack[%d] got size %dx%d, want 100x40", i, fs.width, fs.height)
		}
	}
}
