package pages

// A LIST WITH NOTHING IN IT STILL HAS SOMETHING TO SAY, and noticePanel is the
// one shape all of it takes: a heading and a line under it, on the same panel
// every other surface in the app is made of, centred in the column the cards
// would have filled.
//
// IT IS ONE COMPONENT BECAUSE THERE ARE FOUR OF THESE and they were drifting.
// The two empty states were near-copies of each other; the two "nothing
// matched" messages were bare paragraphs, which is how one of them ended up
// left-aligned on a page whose empty state was centred, and how both of them
// ended up sitting straight on the grid paper with nothing behind them. A
// message that is hard to see is a page that looks broken rather than empty.
//
// THE BODY IS CHILDREN AND NOT A STRING, because one of the four has markup in
// it -- the manual's "run <strong>before</strong> your session" -- and a second
// component taking a template for the body would be the copy this one is
// removing.

// noMatchHint is the line under every "nothing matched" heading, and it is one
// sentence for all of them because the useful thing to say does not vary by
// kind: the list is not empty, the box is filtering it, and the box is where to
// go next. It names no noun, which is what lets maps, tokens, avatars, tracks
// and monsters share it.
const noMatchHint = "Try a shorter term, or clear the search box to see them all."

// noMatchHeading is the sentence a search that found nothing gets, with the
// term repeated back so it is obvious which search is being answered.
//
// THE PLURAL IS PASSED RATHER THAN TAKEN FROM THE KIND, because two of the five
// disagree with their own slug: the music page's is "music", and "No music
// match" is not a sentence -- a track is what one of them is called everywhere
// else in that manager. The manual has no slug here at all.
func noMatchHeading(plural string, query string) string {
	if query == "" {
		return ""
	}

	return "No " + plural + " match \"" + query + "\"."
}
