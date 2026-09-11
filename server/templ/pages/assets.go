package pages

import (
	"strconv"

	"tabletopper/internal/queries"
)

const (
	assetTabMaps    = "maps"
	assetTabTokens  = "tokens"
	assetTabAvatars = "avatars"
	assetTabMusic   = "music"
)
const AssetNameLimit = 255

type MapAsset struct {
	ID         string
	Name       string
	FileName   string
	Generation string
	State      queries.AssetsTileState
	AutoRetry  bool
}

func (m MapAsset) Usable() bool {
	return m.Generation != ""
}
func (m MapAsset) Polling() bool {
	return m.State == queries.AssetsTileStatePending || m.State == queries.AssetsTileStateWorking
}
func (m MapAsset) Retryable() bool {
	return m.State == queries.AssetsTileStateFailed
}
func (m MapAsset) TileFailure() string { return tileFailureText(m.AutoRetry) }
func (m MapAsset) RetryLabel() string  { return retryLabelText(m.AutoRetry) }

const (
	tilingRetrying = "Tiling failed. Trying again in a few minutes."
	tilingGaveUp   = "Tiling gave up."
)

func tileFailureText(autoRetry bool) string {
	if autoRetry {
		return tilingRetrying
	}
	return tilingGaveUp
}
func retryLabelText(autoRetry bool) string {
	if autoRetry {
		return "Try now"
	}
	return "Try again"
}
func (m MapAsset) CardURL() string {
	return "/fragment/assets/maps/" + m.ID + "/card"
}
func (m MapAsset) nameBox() nameBox {
	return nameBox{
		ID:        "map-name-" + m.ID,
		Value:     m.Name,
		URL:       "/assets/maps/" + m.ID + "/name",
		Field:     "map-name",
		MaxLength: strconv.Itoa(AssetNameLimit),
		Size:      nameBoxRoomy,
	}
}
func (m MapAsset) controls() cardControls {
	return cardControls{
		FileName:    m.FileName,
		ReplaceID:   "map-replace-" + m.ID,
		ReplaceURL:  "/assets/maps/" + m.ID,
		Field:       "map",
		Accept:      imageAccept,
		DeleteURL:   "/assets/maps/" + m.ID,
		ConfirmName: m.Name,
	}
}
