package pages

// The three bounds the Vitals panel renders and the handler enforces.
//
// THEY LIVE HERE BECAUSE THE MARKUP NEEDS THEM. A number input's max attribute
// and a handler's range check are the same fact written twice, and the copy in
// the template is the one that drifts silently -- a max nobody updated still
// renders, still validates in the browser, and only disagrees with the server on
// the values a player is least likely to try. controllers already reads
// DefaultAlignment and DefaultSpellSchool out of this package for the same
// reason, and it cannot go the other way: pages cannot import controllers.
//
// Each is a rule rather than a preference. A dying character rolls three death
// saves either way. Exhaustion runs 0 to 6, where 6 is death. And nobody holds
// more hit dice than they have levels, which is why the spent cap is 20 -- the
// xp table in controllers/rules.go stops there, so no character reaches 21.
const (
	DeathSaveLimit    = 3
	ExhaustionLimit   = 6
	HitDiceSpentLimit = 20
)
