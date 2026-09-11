package pages
import "strconv"
type MusicTrack struct {
	ID   string
	Name string
	FileName string
}
func (t MusicTrack) URL() string {
	return "/assets/music/" + t.ID
}
func (t MusicTrack) NameURL() string {
	return t.URL() + "/name"
}
func (t MusicTrack) AudioURL() string {
	return t.URL() + "/audio"
}
func (t MusicTrack) nameBox() nameBox {
	return nameBox{
		ID:        "music-name-" + t.ID,
		Value:     t.Name,
		URL:       t.NameURL(),
		Field:     "name",
		MaxLength: strconv.Itoa(AssetNameLimit),
		Size:      nameBoxRoomy,
	}
}
const MusicAccept = "audio/*,.mp3,.ogg,.oga,.opus,.m4a,.mp4,.webm,.flac,.wav"
