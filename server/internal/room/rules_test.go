package room

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)













type bandCase struct {
	HP    int    `json:"hp"`
	MaxHP int    `json:"maxHp"`
	Band  string `json:"band"`
}

type snapCase struct {
	Cell      int  `json:"cell"`
	Offset    int  `json:"offset"`
	Footprint int  `json:"footprint"`
	Mode      Snap `json:"mode"`
	Value     int  `json:"value"`
	Snapped   int  `json:"snapped"`
}

type hpCase struct {
	Entry   string `json:"entry"`
	Current *int   `json:"current"`
	Value   *int   `json:"value"`
	Refused bool   `json:"refused"`
}

func TestRuleFixturesAreCurrent(t *testing.T) {
	t.Run("bands", func(t *testing.T) {
		var cases []bandCase
		for _, maxHP := range []int{1, 2, 3, 7, 10, 13, 20, 100} {
			for hp := 0; hp <= maxHP; hp++ {
				cases = append(cases, bandCase{HP: hp, MaxHP: maxHP, Band: string(*hpBand(&hp, &maxHP))})
			}
		}
		pinFixture(t, "bands", cases)
	})

	t.Run("snap", func(t *testing.T) {
		var cases []snapCase
		for _, cell := range []int{50, 64} {
			for _, offset := range []int{-12, 0, 8} {
				for _, footprint := range []int{1, 2, 3} {
					for _, mode := range []Snap{SnapOff, SnapCells, SnapHalfCells} {
						for _, v := range []int{-137, -100, -33, -32, -31, -1, 0, 1, 15, 16, 17, 31, 32, 33, 49, 50, 99, 1234} {
							cases = append(cases, snapCase{
								Cell: cell, Offset: offset, Footprint: footprint, Mode: mode, Value: v,
								Snapped: SnapAxis(cell, offset, footprint, mode, v),
							})
						}
					}
				}
			}
		}
		pinFixture(t, "snap", cases)
	})

	t.Run("hp", func(t *testing.T) {
		twelve := 12
		var cases []hpCase
		for _, entry := range []string{
			"12", "0", "-7", "+3", "  -7  ", "12-7", "12-7-4", "12 - 7", "-7-4", "-99", "+0",
			"lots", "7hp", "--7", "12-", "+", "1+2+3+4+5+6+7+8+9+10+11+12",
		} {
			for _, current := range []*int{&twelve, nil} {
				c := hpCase{Entry: entry, Current: current}
				value, present, refusal := EvaluateHP(entry, current)
				switch {
				case refusal != "":
					c.Refused = true
				case present:
					v := value
					c.Value = &v
				}
				cases = append(cases, c)
			}
		}
		pinFixture(t, "hp", cases)
	})
}



func pinFixture(t *testing.T, name string, cases any) {
	t.Helper()

	got, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", "rules", name+".json")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v -- run `go test ./internal/room -update` to write it", err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s is stale; run `go test ./internal/room -update` and read the diff", path)
	}
}
