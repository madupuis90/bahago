package guild

// MemberRole represents a kingdom's role within a guild.
type MemberRole string

const (
	RoleNone            MemberRole = ""
	RoleApplicant       MemberRole = "applicant"
	RoleSupporter       MemberRole = "supporter"
	RolePendingApproval MemberRole = "pending_approval"
	RoleMember          MemberRole = "member"
	RoleOfficer         MemberRole = "officer"
	RoleLeader          MemberRole = "leader"
	// RoleInOtherGuild is a pseudo-role used only in the view layer to indicate the
	// viewer has an active commitment to a different guild. Never stored in the database.
	RoleInOtherGuild MemberRole = "in_other_guild"
)

// IsActiveMember reports whether the role counts as a full guild member
// (i.e. the kingdom holds a seat, not merely an application-phase role).
func (r MemberRole) IsActiveMember() bool {
	return r == RoleMember || r == RoleOfficer || r == RoleLeader
}

// IsApplicationPhase reports whether the role belongs to the guild's application phase.
func (r MemberRole) IsApplicationPhase() bool {
	return r == RoleApplicant || r == RoleSupporter
}

// CanManage reports whether the role has officer-level management access.
func (r MemberRole) CanManage() bool {
	return r == RoleOfficer || r == RoleLeader
}

// CanRemoveTarget reports whether the role can remove or demote a guild member
// with the given target role. Leaders can act on officers and members; officers
// can only act on regular members.
func (r MemberRole) CanRemoveTarget(target MemberRole) bool {
	switch r {
	case RoleLeader:
		return target == RoleMember || target == RoleOfficer
	case RoleOfficer:
		return target == RoleMember
	default:
		return false
	}
}

// IsLeader reports whether the role is the guild leader.
func (r MemberRole) IsLeader() bool {
	return r == RoleLeader
}

// Display returns a human-readable label for the role.
func (r MemberRole) Display() string {
	switch r {
	case RoleApplicant:
		return "Applicant"
	case RoleSupporter:
		return "Supporter"
	case RolePendingApproval:
		return "Pending Approval"
	case RoleMember:
		return "Member"
	case RoleOfficer:
		return "Officer"
	case RoleLeader:
		return "Leader"
	default:
		return string(r)
	}
}

// GuildStatus represents the lifecycle state of a guild.
type GuildStatus string

const (
	GuildStatusPending GuildStatus = "pending"
	GuildStatusActive  GuildStatus = "active"
)

// IsPending reports whether the guild is in the application phase.
func (s GuildStatus) IsPending() bool {
	return s == GuildStatusPending
}

// IsActive reports whether the guild is fully active.
func (s GuildStatus) IsActive() bool {
	return s == GuildStatusActive
}
