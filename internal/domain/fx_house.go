package domain

import "fmt"

// FxHouse is which dollar quote a period's rate is drawn from — the
// Argentine parallel markets track several, at different spreads.
type FxHouse int

const (
	Blue FxHouse = iota
	Official
	MEP
)

func (h FxHouse) String() string {
	switch h {
	case Blue:
		return "Blue"
	case Official:
		return "Official"
	case MEP:
		return "MEP"
	default:
		return fmt.Sprintf("FxHouse(%d)", int(h))
	}
}

func ParseFxHouse(s string) (FxHouse, error) {
	switch s {
	case "Blue":
		return Blue, nil
	case "Official":
		return Official, nil
	case "MEP":
		return MEP, nil
	default:
		return 0, fmt.Errorf("domain: invalid fx house %q", s)
	}
}
