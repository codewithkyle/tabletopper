package room
type Role string
const (
	RoleGM Role = "gm"
	RolePlayer Role = "player"
)
func (Role) Values() []string { return []string{string(RoleGM), string(RolePlayer)} }
func (r Role) Valid() bool { return inValues(r, r.Values()) }
