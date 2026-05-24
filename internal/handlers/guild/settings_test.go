package guild

import (
	"errors"
	"testing"

	_guild "bahago/internal/guild"
)

// ── validateSettingsInput ─────────────────────────────────────────────────────

func TestValidateSettingsInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *settingsSignals
		wantErrs []error
	}{
		{"valid_member_member", &settingsSignals{GuildMsgAll: string(_guild.RoleMember), GuildMsgOfficers: string(_guild.RoleMember)}, nil},
		{"valid_leader_only", &settingsSignals{GuildMsgAll: string(_guild.RoleLeader), GuildMsgOfficers: string(_guild.RoleLeader)}, nil},
		{"all_invalid", &settingsSignals{GuildMsgAll: "bogus", GuildMsgOfficers: string(_guild.RoleMember)}, []error{ErrInvalidPermissionValue}},
		{"officers_invalid", &settingsSignals{GuildMsgAll: string(_guild.RoleMember), GuildMsgOfficers: ""}, []error{ErrInvalidPermissionValue}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateSettingsInput(tc.input)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}
