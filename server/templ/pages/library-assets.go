package pages
import "strconv"
type LibraryAsset struct {
	ID   string
	Name string
	FileName string
	Kind string
	Width  int
	Height int
}
func (l LibraryAsset) URL() string {
	return "/assets/" + l.Kind + "/" + l.ID
}
func (l LibraryAsset) NameURL() string {
	return l.URL() + "/name"
}
func (l LibraryAsset) ImageURL() string {
	return "/assets/images/" + l.ID
}
func (l LibraryAsset) nameBox() nameBox {
	size := nameBoxRoomy
	if l.Kind == assetTabAvatars {
		size = nameBoxTight
	}
	return nameBox{
		ID:        l.Kind + "-name-" + l.ID,
		Value:     l.Name,
		URL:       l.NameURL(),
		Field:     "name",
		MaxLength: strconv.Itoa(AssetNameLimit),
		Size:      size,
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
