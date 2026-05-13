package guild

import "encoding/json"

// MessagePermissions holds the minimum role required to use each guild-message
// recipient group. Values are stored as JSONB in guilds.settings.
type MessagePermissions struct {
	MsgAll      MemberRole `json:"guild_msg_all"`      // min role to message all members
	MsgOfficers MemberRole `json:"guild_msg_officers"` // min role to message officers only
}

// DefaultMessagePermissions defines the values used when settings keys are absent.
var DefaultMessagePermissions = MessagePermissions{
	MsgAll:      RoleOfficer,
	MsgOfficers: RoleMember,
}

// ParseMessagePermissions reads the JSONB blob from guilds.settings into a
// MessagePermissions value. Missing or invalid keys fall back to the defaults.
func ParseMessagePermissions(raw []byte) MessagePermissions {
	p := DefaultMessagePermissions
	if len(raw) == 0 {
		return p
	}
	var parsed MessagePermissions
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return p
	}
	if parsed.MsgAll.Rank() > 0 {
		p.MsgAll = parsed.MsgAll
	}
	if parsed.MsgOfficers.Rank() > 0 {
		p.MsgOfficers = parsed.MsgOfficers
	}
	return p
}

// CanSendToAll reports whether role is permitted to send a guild message to
// all active members, according to this guild's configured permissions.
func (p MessagePermissions) CanSendToAll(role MemberRole) bool {
	return role.AtLeast(p.MsgAll)
}

// CanSendToOfficers reports whether role is permitted to send a guild message
// to officers and the leader only.
func (p MessagePermissions) CanSendToOfficers(role MemberRole) bool {
	return role.AtLeast(p.MsgOfficers)
}

// CanSendAny reports whether role is permitted to use at least one guild-message
// recipient group.
func (p MessagePermissions) CanSendAny(role MemberRole) bool {
	return p.CanSendToOfficers(role) || p.CanSendToAll(role)
}
