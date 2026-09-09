package pages

// THE FLOOR SWAP IN THE TOOL PILL, AND WHY A THIRD FLOOR CONTROL IS NOT ONE
// TOO MANY. There are two already and they answer different questions. The bar
// picks the floor THIS GM IS LOOKING AT, which is a local choice nobody else
// can see; the layers window is where floors are made, named, reordered and
// deleted. Neither is the one thing a GM does over and over while running a
// fight -- put the party on the roof and let the players see the roof -- and
// doing that meant opening a window, finding a radio button in a row of five
// controls, and closing the window again, every time the party used a staircase.
//
// SO THIS IS THE ACTIVE LAYER AND ONLY THE ACTIVE LAYER. One list, one click,
// one thing that happens: the floor everybody at the table is shown. It is the
// same POST the radio in the layers window makes, so there is one route, one
// event and one story about what changed.
//
// THE GM'S OWN VIEW FOLLOWS AND IS NOT SET HERE. Making a floor active clears
// the local override in render/layers.ts, which is a rule that exists for this
// exact gesture: a GM who has wandered off to the cellar and then puts the
// party on the roof means to be looking at the roof.
//
// IT IS THE GM'S OR IT IS NOTHING. A player cannot activate a layer -- the
// route refuses them and the projection never sends them another floor's pawns
// -- so their page renders neither the button nor the menu, and the client
// finds nothing to mount rather than a control it has to hide.
//
// THE MENU IS A SIBLING OF THE TABLE AND NOT A CHILD OF THE PILL, which is the
// same arrangement the right-click menu has and for the same two reasons. The
// pill is positioned and z-indexed, so it is a stacking context, and a menu
// inside it could never rise above a window that happened to be under the
// corner; and a DaisyUI dropdown inside the tooltip wrapper every other button
// in the pill has would put a tooltip over its own list. It is placed by
// server/js/room/layer-tool.ts, which anchors it to the button and raises it
// above every window.
//
// THE ROWS ARE FILLED FROM THE STORE RATHER THAN FETCHED, for the reason the
// bar's select is: floors are added, renamed and deleted while the table is
// live, and the list is a few names long. It is rebuilt every time the menu
// opens, so nothing is stale and nothing has to be invalidated. The heading is
// kept and the rows are replaced, which is why it is the one element in here
// with a data attribute of its own.
