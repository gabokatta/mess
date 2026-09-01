package project

import "testing"

func TestCheckboxesFindsEachTaskItemInOrder(t *testing.T) {
	body := "## ARG\n- [ ] book flights\n- [x] renew passport\n\nsome prose in between\n\n- [ ] buy pesos"

	got := Checkboxes(body)

	want := []Checkbox{
		{Text: "book flights", Done: false},
		{Text: "renew passport", Done: true},
		{Text: "buy pesos", Done: false},
	}
	if len(got) != len(want) {
		t.Fatalf("Checkboxes() = %+v, want %d items", got, len(want))
	}
	for i, w := range want {
		if got[i].Text != w.Text || got[i].Done != w.Done {
			t.Errorf("Checkboxes()[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestCheckboxesIgnoresPlainListItems(t *testing.T) {
	body := "- just a note\n- [ ] a real task"

	got := Checkboxes(body)

	if len(got) != 1 || got[0].Text != "a real task" {
		t.Errorf("Checkboxes() = %+v, want only the real task", got)
	}
}

func TestToggleFlipsTheNthCheckboxOnly(t *testing.T) {
	body := "- [ ] book flights\n- [ ] renew passport"

	got := Toggle(body, 1)

	want := "- [ ] book flights\n- [x] renew passport"
	if got != want {
		t.Errorf("Toggle(body, 1) = %q, want %q", got, want)
	}

	got = Toggle(got, 1)
	if got != body {
		t.Errorf("Toggle() twice = %q, want back to %q", got, body)
	}
}

func TestToggleOutOfRangeIsANoop(t *testing.T) {
	body := "- [ ] book flights"

	if got := Toggle(body, 5); got != body {
		t.Errorf("Toggle() with an out-of-range index = %q, want body unchanged", got)
	}
}

func TestProgressCountsDoneAgainstTotal(t *testing.T) {
	body := "- [x] book flights\n- [ ] renew passport\n- [x] buy pesos"

	done, total := Progress(body)

	if done != 2 || total != 3 {
		t.Errorf("Progress() = %d/%d, want 2/3", done, total)
	}
}
