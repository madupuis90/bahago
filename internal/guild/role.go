package guild

// MemberRole represents a kingdom's role within a guild.
type MemberRole string

const (
	RoleNone            MemberRole = ""
	RoleApplicant       MemberRole = "applicant"
	RoleSupporter       MemberRole = "supporter"
	RolePendingApproval MemberRole = "pending_approval"
	RoleInvited         MemberRole = "invited"
	RoleMember          MemberRole = "member"
	RoleOfficer         MemberRole = "officer"
	RoleLeader          MemberRole = "leader"

	// RoleInOtherGuild is a view-layer pseudo-role used only when rendering a
	// guild page for a kingdom that is committed to a different guild. It is
	// never stored in the database.
	RoleInOtherGuild MemberRole = "in_other_guild"
)

// Rank returns the seniority level of the role within the active-member hierarchy.
// Only member, officer, and leader carry a rank; all other roles return 0.
func (r MemberRole) Rank() int {
	switch r {
	case RoleMember:
		return 1
	case RoleOfficer:
		return 2
	case RoleLeader:
		return 3
	default:
		return 0
	}
}

// AtLeast reports whether the role's rank is at least as high as min's rank.
// Roles with rank 0 (non-active) always return false.
func (r MemberRole) AtLeast(min MemberRole) bool {
	return r.Rank() > 0 && r.Rank() >= min.Rank()
}

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

// CanReceiveInvitation reports whether the role allows receiving a guild invitation.
// A kingdom can be invited if it has no conflicting membership in this guild.
func (r MemberRole) CanReceiveInvitation() bool {
	return r == RoleNone || r == RoleInOtherGuild || r == RoleInvited
}

// CanLeave reports whether the role allows a kingdom to voluntarily leave a guild.
func (r MemberRole) CanLeave() bool {
	return r == RoleMember || r == RoleOfficer
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
	case RoleInvited:
		return "Invited"
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
