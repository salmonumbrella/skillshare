package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type filterKeyFixture struct {
	filtering bool
	text      string
	applied   int
}

func newFilterKeyFixture(text string) *filterKeyFixture {
	return &filterKeyFixture{filtering: true, text: text}
}

func (f *filterKeyFixture) press(t *testing.T, msg tea.KeyMsg, value string) {
	t.Helper()
	input := newTUIFilterInput("")
	input.SetValue(value)
	input.Focus()
	handleTUIFilterKey(msg, &f.filtering, &f.text, &input, func() { f.applied++ })
}

func TestHandleTUIFilterKey_EscClearsAndApplies(t *testing.T) {
	f := newFilterKeyFixture("abc")
	f.press(t, tea.KeyMsg{Type: tea.KeyEsc}, "abc")

	if f.filtering || f.text != "" || f.applied != 1 {
		t.Fatalf("got filtering=%v text=%q applied=%d", f.filtering, f.text, f.applied)
	}
}

func TestHandleTUIFilterKey_EnterStopsWithoutApplying(t *testing.T) {
	f := newFilterKeyFixture("abc")
	f.press(t, tea.KeyMsg{Type: tea.KeyEnter}, "abc")

	if f.filtering || f.text != "abc" || f.applied != 0 {
		t.Fatalf("got filtering=%v text=%q applied=%d", f.filtering, f.text, f.applied)
	}
}

func TestHandleTUIFilterKey_TypingAppliesOnce(t *testing.T) {
	f := newFilterKeyFixture("ab")
	f.press(t, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}, "ab")

	if !f.filtering || f.text != "abc" || f.applied != 1 {
		t.Fatalf("got filtering=%v text=%q applied=%d", f.filtering, f.text, f.applied)
	}
}

func TestHandleTUIFilterKey_UnchangedValueDoesNotApply(t *testing.T) {
	f := newFilterKeyFixture("ab")
	f.press(t, tea.KeyMsg{Type: tea.KeyLeft}, "ab")

	if f.text != "ab" || f.applied != 0 {
		t.Fatalf("got text=%q applied=%d", f.text, f.applied)
	}
}
