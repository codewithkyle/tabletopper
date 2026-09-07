package pages

// The manual page's cards. A card is one row, and the shape of it is a bet
// about how this page gets used: a GM accumulates monsters for years, so the
// question the manual answers is "which of these hundred" rather than "tell me
// about this one". A row that fits forty on a screen answers it; a tile that
// fits eight does not.
//
// SO THE CARD CARRIES THREE NUMBERS AND NOT SIX. The challenge rating first,
// because it is the one that answers "can I put this in front of them tonight",
// then AC and hit points. Speed came off with the second column -- it settles no
// question worth scanning a hundred rows for, and the quick view is one click
// away for the times it does.
//
// ALL OF THE REASONING ABOUT THIS CARD LIVES IN THIS FILE, and it has to. The
// markup is monsters.templ, which cannot carry a comment of any kind -- Tailwind
// reads every .templ file as text and takes a class-name candidate from anything
// word-shaped it finds, so an ordinary English sentence containing "table" or
// "stack" emits a DaisyUI component family into the built stylesheet and nothing
// fails.

// monsterCardsID is the grid the search box swaps and the id it points its
// aria-controls at. It is written once here because three attributes in two
// components read it, and a box aimed at an element that is not there fails
// silently -- htmx logs to the console and the list simply never changes.
const monsterCardsID = "monster-cards"

// MonsterListData is the manual's list in whichever of its two states it is in:
// everything the owner has, or what matched a search.
//
// QUERY IS ON HERE FOR THE EMPTY STATE AND NOTHING ELSE. A manual with nothing
// in it and a search that found nothing are different things to say to somebody
// -- one wants the New Monster button pointed out and the other wants the term
// repeated back -- and the list cannot tell them apart from its own length.
type MonsterListData struct {
	Monsters []MonsterSummary
	Query    string
}

// MonsterSummary is one monster as its card needs it, and no more of the row
// than that. The manual lists hundreds of these and the card shows five values,
// so it takes the five rather than the whole stat block -- which also means the
// search fragment and the page render the same struct, filled in by the same
// function.
//
// Subtitle arrives already assembled, the way CharacterHeader's does, because
// the controller is the only place that knows which of size, type, tags and
// alignment the row actually has.
type MonsterSummary struct {
	ID       string
	Name     string
	Subtitle string
	// ImageID is empty when the monster has no picture. Either way the card
	// carries the upload control, which is MonsterImageControl and is the same
	// one the editor's bar carries -- see monster-image.go.
	ImageID string

	// The three chips, each already the string it prints. CR is the rating on
	// its own; the XP and the proficiency bonus that follow from it are stat
	// block readings and would be three numbers on a card that has room for one.
	CR string
	AC string
	HP string
}
