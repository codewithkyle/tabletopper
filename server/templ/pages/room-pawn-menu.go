package pages

// THE RIGHT BUTTON ON A PAWN PUTS UP THIS MENU, AND IT USED TO OPEN THE PAWN'S
// WINDOW. The change came out of playing at a real table: people reached for a
// double click to open a pawn without being told to, found nothing under it,
// and never discovered the right click that did work. So the double click is
// what opens the window now -- see pawns.ts -- and the right button asks a
// question instead, with Open details as the first answer on the list. The
// gesture people looked for works, and the one they were taught still gets
// them there.
//
// WHAT IS ON IT IS THE THREE THINGS THAT ARE ABOUT ONE PAWN. Opening it is
// everybody's. Moving it to another floor and taking it off the table are the
// GM's, and both of those previously had nowhere to be asked for: the floor
// select and the Remove button in the overlay only appear for a selection of
// several, and the Delete key acts on the selection and needs a keyboard.
// Right-clicking the goblin you are looking at is how somebody asks about the
// goblin they are looking at.
//
// IT IS A MENU AND NOT A WINDOW, so it is dismissed by the next thing the hand
// does rather than by a control of its own. That is what every context menu on
// every platform does, it is why there is no ✕ on it, and it is why the modal
// rule about a labelled Close is not in play: nothing here interrupts anything.
//
// THE TWO MUTATIONS ARE ORDINARY htmx BUTTONS AND NOT A FETCH IN THE CLIENT.
// Removing pawns is confirmed, the confirmation is the app's confirm modal, and
// hx-confirm is read off the element that makes the request -- so a DELETE
// built in JavaScript would skip the dialog, and window.confirm is banned. The
// floor move is the same button pattern for consistency, and it posts to the
// same route the overlay's whole-selection control does: one route, a list of
// one.
//
// THE REMOVE BUTTON IS VISIBLE AND THE MOVE BUTTON IS NOT, which looks
// inconsistent and is not. Remove is an item on the list and belongs on it. The
// floor items are cloned per layer, and a cloned element has never been through
// htmx, so it cannot carry hx-post -- it carries a floor id, and pressing it
// writes that id onto the one button that HAS been processed. That button is
// the same device roomRemoveKey is: an element for a request to belong to,
// never seen.
//
// THE LAYER ROWS ARE A <template> FOR THE REASON A WINDOW'S CHROME IS ONE.
// server/js is not a Tailwind source, so a class name written in pawn-menu.ts
// would never be emitted; the row is styled here and cloned there, and the only
// things the client writes into it are text, a data attribute and [hidden].
//
// AND THE FLOOR IT IS ALREADY ON SAYS SO. A list of five names with nothing to
// tell them apart makes the GM open the pawn's window to find out where it is
// before they can decide where to send it, which is the window this menu exists
// to save them.
