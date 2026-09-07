package pages

import "strconv"

// THE LIBRARY IS EVERY KIND THE ACCOUNT GATHERS RATHER THAN A SHEET OWNS, and
// one type covers all of them because they differ in nothing a card can see.
//
// A token and an avatar are one stored image, a name, and the file they came
// from. They are told apart by the segment their routes sit under, which is the
// only reason Kind is on here at all -- the card builds its URLs from it, so a
// token's Delete goes to the tokens route and cannot be aimed at a map.
//
// THIS IS NOT MapAsset AND SHOULD NOT BECOME IT. A map is a pyramid, a
// generation and a background job as well as a row, and MapAsset carries three
// questions about how those disagree with each other. None of them means
// anything here: a library asset is either in the bucket or was never uploaded,
// and its card never polls. Folding the two together would put three methods
// that are always false onto every token in the manager.
type LibraryAsset struct {
	ID   string
	Name string
	// FileName is what the file was called when it was uploaded, shown on the
	// card as a chip. It is not the name and is not editable.
	FileName string
	// Kind is the path segment this asset's routes live under: "tokens",
	// "avatars". It is the plural, because it names the collection.
	Kind string
	// Width and Height are the STORED image's pixels, which for a token is the
	// shape it was drawn at and for an avatar is a square. Nothing on the card
	// reads them today; they are here because the page-data type is what the
	// controller fills in, and a renderer placing a token needs its aspect
	// without fetching the file to measure it.
	Width  int
	Height int
}

// URL is this asset's own resource, which its Replace and Delete both post to.
// It is a method for the reason MapAsset.CardURL is one: a path assembled in a
// .templ file is one nothing can point at from routes.go, and a card aimed at a
// route that no longer exists fails silently -- the swap simply never happens.
func (l LibraryAsset) URL() string {
	return "/assets/" + l.Kind + "/" + l.ID
}

// NameURL is the name resource, which is a PATCH of its own rather than part of
// the replace: renaming does not touch the object, and re-uploading does not
// touch the name.
func (l LibraryAsset) NameURL() string {
	return l.URL() + "/name"
}

// ImageURL is the stored image itself.
//
// IT IS NOT THE /preview VARIANT, and the difference is that a library asset has
// no second size to serve. A map is stored huge and its card shows a thumbnail
// built by the tiler; a token and an avatar are stored at the size they are
// used at, so preview_path is NULL on the row and the preview route would fall
// back to this same object anyway.
func (l LibraryAsset) ImageURL() string {
	return "/assets/images/" + l.ID
}

// nameBox and controls are what this asset gives the two shared card
// components. See asset-card.go for why they are structs built here rather than
// arguments assembled in the markup.
func (l LibraryAsset) nameBox() nameBox {
	return nameBox{
		ID:        l.Kind + "-name-" + l.ID,
		Value:     l.Name,
		URL:       l.NameURL(),
		Field:     "name",
		MaxLength: strconv.Itoa(AssetNameLimit),
	}
}

func (l LibraryAsset) controls() cardControls {
	return cardControls{
		FileName:    l.FileName,
		ReplaceID:   l.Kind + "-replace-" + l.ID,
		ReplaceURL:  l.URL(),
		Field:       "image",
		Accept:      imageAccept,
		DeleteURL:   l.URL(),
		ConfirmName: l.Name,
	}
}
