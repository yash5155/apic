package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yash5155/apic/internal/spec"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestNewFieldPicksEnum(t *testing.T) {
	enum := newField(spec.Param{Name: "status", In: "query", Enum: []string{"a", "b"}})
	if _, ok := enum.(*enumField); !ok {
		t.Errorf("enum param should be enumField, got %T", enum)
	}
	text := newField(spec.Param{Name: "id", In: "path"})
	if _, ok := text.(*textField); !ok {
		t.Errorf("plain param should be textField, got %T", text)
	}
}

func TestEnumFieldCyclesAndIgnoresTyping(t *testing.T) {
	e := newField(spec.Param{Name: "status", In: "query", Enum: []string{"available", "pending", "sold"}})
	e.Focus()

	// Starts empty (unchosen), so a required enum is caught by missingRequired.
	if e.Value() != "" {
		t.Errorf("enum should start empty, got %q", e.Value())
	}

	// Typing letters must not set a value.
	e, _ = e.Update(key("x"))
	if e.Value() != "" {
		t.Errorf("typing should be ignored, got %q", e.Value())
	}

	// right cycles forward through the options.
	e, _ = e.Update(key("right"))
	if e.Value() != "available" {
		t.Errorf("after right = %q, want available", e.Value())
	}
	e, _ = e.Update(key("right"))
	if e.Value() != "pending" {
		t.Errorf("after right x2 = %q, want pending", e.Value())
	}

	// left wraps back to the empty option.
	e, _ = e.Update(key("left"))
	e, _ = e.Update(key("left"))
	if e.Value() != "" {
		t.Errorf("after wrapping left = %q, want empty", e.Value())
	}
}

func TestEnumFieldSetValue(t *testing.T) {
	e := newField(spec.Param{Name: "s", Enum: []string{"a", "b", "c"}})
	e.SetValue("c")
	if e.Value() != "c" {
		t.Errorf("SetValue(c) then Value = %q", e.Value())
	}
	// An invalid value is ignored (can't set what isn't an option).
	e.SetValue("zzz")
	if e.Value() != "c" {
		t.Errorf("invalid SetValue should be a no-op, got %q", e.Value())
	}
}
