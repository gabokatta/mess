package note

import "testing"

const body = `# Groceries

- [ ] milk
- [x] bread
  * [ ] the good one
- not a checkbox
`

func TestCheckboxes(t *testing.T) {
	got := Checkboxes(body)

	want := []Checkbox{
		{Text: "milk", Done: false},
		{Text: "bread", Done: true},
		{Text: "the good one", Done: false},
	}
	if len(got) != len(want) {
		t.Fatalf("Checkboxes() returned %d items, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Checkboxes()[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestProgress(t *testing.T) {
	done, total := Progress(body)
	if done != 1 || total != 3 {
		t.Errorf("Progress() = %d/%d, want 1/3", done, total)
	}
}

func TestToggleRewritesOnlyTheTargetLine(t *testing.T) {
	got := Toggle(body, 0)

	want := `# Groceries

- [x] milk
- [x] bread
  * [ ] the good one
- not a checkbox
`
	if got != want {
		t.Errorf("Toggle(0) =\n%s\nwant\n%s", got, want)
	}
}

func TestToggleUnticks(t *testing.T) {
	boxes := Checkboxes(Toggle(body, 1))
	if boxes[1].Done {
		t.Error("Toggle() on a ticked box should untick it")
	}
}

// A cursor that outran the list must never corrupt the body.
func TestToggleOutOfRangeIsANoOp(t *testing.T) {
	if got := Toggle(body, 9); got != body {
		t.Errorf("Toggle(9) changed the body:\n%s", got)
	}
}
