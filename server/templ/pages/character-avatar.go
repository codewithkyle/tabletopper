package pages

import "tabletopper/internal/queries"

type CharacterAvatar struct {
	CharacterID string
	Name        string
	AvatarID    string
}

func (c CharacterAvatar) Verb() string {
	if c.AvatarID == "" {
		return "Upload"
	}
	return "Replace"
}
func characterAvatarInputID(characterID string) string {
	return "character-avatar-" + characterID
}
func characterAvatar(character queries.Character) CharacterAvatar {
	avatar := CharacterAvatar{CharacterID: character.ID.String(), Name: character.Name}
	if character.AssetID != nil {
		avatar.AvatarID = character.AssetID.String()
	}
	return avatar
}
