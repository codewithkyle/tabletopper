package pages

import (
	"strings"
	"testing"

	"tabletopper/internal/dnd"

	"github.com/a-h/templ"
)

const testPurseCharacterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func testPurse() CoinPurse {
	var purse dnd.Purse
	purse[dnd.Silver] = 5
	purse[dnd.Gold] = 97
	return NewCoinPurse(testPurseCharacterID, purse)
}
func TestCoinTotalLabelCountsInGold(t *testing.T) {
	for _, c := range []struct {
		purse dnd.Purse
		want  string
	}{
		{dnd.Purse{}, "0 gp"},
		{dnd.Purse{5, 0, 0, 0, 0}, "0.05 gp"},
		{dnd.Purse{0, 5, 0, 0, 0}, "0.5 gp"},
		{dnd.Purse{0, 0, 1, 0, 0}, "0.5 gp"},
		{dnd.Purse{3, 5, 0, 97, 0}, "97.53 gp"},
		{dnd.Purse{0, 0, 0, 0, 1}, "10 gp"},
	} {
		if got := CoinTotalLabel(c.purse); got != c.want {
			t.Errorf("CoinTotalLabel(%v) = %q, want %q", c.purse, got, c.want)
		}
	}
}
func TestTheInventoryPageAndItsWindowCarryTheSamePurse(t *testing.T) {
	data := InventoryPageData{CharacterID: testPurseCharacterID, Purse: testPurse()}
	for name, page := range map[string]templ.Component{
		"page":   EditCharacterInventory(data),
		"window": CharacterSheetWindow(SheetWindowData{Section: SheetSectionInventory, Inventory: data}),
	} {
		body := renderString(t, page)
		for _, want := range []string{
			`class="` + inventoryLayout + `"`,
			`hx-post="/characters/` + testPurseCharacterID + `/coins/change"`,
			`hx-post="/characters/` + testPurseCharacterID + `/coins"`,
			`name="change"`,
			`name="copper"`, `name="silver"`, `name="electrum"`, `name="gold"`, `name="platinum"`,
			`value="97"`, `value="5"`,
			"97.5 gp",
			"@max-[900px]/sheet:order-first",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("the %s is missing %s", name, want)
			}
		}
		if inventory, purse := strings.Index(body, ">Inventory<"), strings.Index(body, ">Coin Purse<"); inventory < 0 || purse < inventory {
			t.Errorf("the %s does not put the purse after the inventory, where the wide layout gives it the right-hand column", name)
		}
	}
}
func TestTheQuickChangeBoxKeepsARefusedEntryAndSaysWhy(t *testing.T) {
	refused := CoinPurse{CharacterID: testPurseCharacterID, Change: "-25", Problem: "Write each amount with its coin."}
	body := renderString(t, CoinChangeForm(refused))
	for _, want := range []string{`value="-25"`, `aria-invalid="true"`, "input-error", `role="alert"`, "Write each amount with its coin."} {
		if !strings.Contains(body, want) {
			t.Errorf("the refused box is missing %s:\n%s", want, body)
		}
	}
	clean := renderString(t, CoinChangeForm(CoinPurse{CharacterID: testPurseCharacterID}))
	for _, unwanted := range []string{"aria-invalid", "input-error", `role="alert"`} {
		if strings.Contains(clean, unwanted) {
			t.Errorf("a fresh box carries %s:\n%s", unwanted, clean)
		}
	}
}
func TestAReadOnlySheetShowsOnlyTheCoinsHeld(t *testing.T) {
	var purse dnd.Purse
	purse[dnd.Gold] = 97
	sheet := testSharedSheet()
	sheet.Coins = HeldCoins(purse)
	sheet.CoinTotal = CoinTotalLabel(purse)
	view := testCharacterView(SheetSectionInventory)
	view.Sheet = sheet
	for name, page := range map[string]templ.Component{
		"GM view":      characterViewSection(view),
		"shared sheet": SharedCharacterPage(sheet),
	} {
		body := renderString(t, page)
		for _, want := range []string{">Coin Purse<", ">gp<", ">97<", "97 gp"} {
			if !strings.Contains(body, want) {
				t.Errorf("the %s is missing %s", name, want)
			}
		}
		for _, unwanted := range []string{">sp<", ">cp<", `name="gold"`} {
			if strings.Contains(body, unwanted) {
				t.Errorf("the %s shows %s", name, unwanted)
			}
		}
	}
	view.Sheet.Coins = HeldCoins(dnd.Purse{})
	if body := renderString(t, characterViewSection(view)); strings.Contains(body, "Coin Purse") {
		t.Error("an empty purse still draws a panel")
	}
}
