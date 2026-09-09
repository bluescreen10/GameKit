package gamekit

import (
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

func TestKeyStateUpdatesBeforeCallback(t *testing.T) {
	w := &Window{}
	var actions []keyboard.KeyAction
	w.SetKeyCallback(func(gotWindow *Window, key keyboard.Key, _ int, action keyboard.KeyAction, _ keyboard.ModifierKey) {
		if gotWindow != w {
			t.Fatal("callback received a different window")
		}
		if got := w.GetKey(key); got != keyboard.KeyPressed {
			t.Fatalf("GetKey during %v callback = %v, want KeyPressed", action, got)
		}
		actions = append(actions, action)
	})

	w.dispatchKey(keyboard.KeyW, 0, keyboard.KeyPressed, 0)
	w.dispatchKey(keyboard.KeyW, 0, keyboard.KeyPressed, 0)
	w.dispatchKey(keyboard.KeyW, 0, keyboard.KeyRepeat, 0)

	if got := w.GetKey(keyboard.KeyW); got != keyboard.KeyPressed {
		t.Fatalf("held key state = %v, want KeyPressed", got)
	}
	want := []keyboard.KeyAction{keyboard.KeyPressed, keyboard.KeyRepeat, keyboard.KeyRepeat}
	if len(actions) != len(want) {
		t.Fatalf("callback actions = %v, want %v", actions, want)
	}
	for i := range want {
		if actions[i] != want[i] {
			t.Fatalf("callback actions = %v, want %v", actions, want)
		}
	}

	w.SetKeyCallback(nil)
	w.dispatchKey(keyboard.KeyW, 0, keyboard.KeyReleased, 0)
	if got := w.GetKey(keyboard.KeyW); got != keyboard.KeyReleased {
		t.Fatalf("released key state = %v, want KeyReleased", got)
	}
}

func TestPointerButtonStateUpdatesBeforeCallback(t *testing.T) {
	w := &Window{}
	called := false
	w.SetPointerButtonCallback(func(gotWindow *Window, button pointer.Button, action pointer.ButtonAction, _ keyboard.ModifierKey) {
		called = true
		if gotWindow != w {
			t.Fatal("callback received a different window")
		}
		if got := w.GetPointerButton(button); got != action {
			t.Fatalf("GetPointerButton during callback = %v, want %v", got, action)
		}
	})

	w.dispatchPointerButton(pointer.ButtonLeft, pointer.Pressed, 0)
	if !called {
		t.Fatal("pointer callback was not called")
	}
	if got := w.GetPointerButton(pointer.ButtonLeft); got != pointer.Pressed {
		t.Fatalf("held button state = %v, want Pressed", got)
	}

	w.dispatchPointerButton(pointer.ButtonLeft, pointer.Released, 0)
	if got := w.GetPointerButton(pointer.ButtonLeft); got != pointer.Released {
		t.Fatalf("released button state = %v, want Released", got)
	}
}

func TestInputStateRejectsUnknownValues(t *testing.T) {
	w := &Window{}
	if got := w.GetKey(keyboard.KeyUnknown); got != keyboard.KeyReleased {
		t.Fatalf("unknown key state = %v, want KeyReleased", got)
	}
	if got := w.GetPointerButton(pointer.Button(-1)); got != pointer.Released {
		t.Fatalf("unknown button state = %v, want Released", got)
	}
}
