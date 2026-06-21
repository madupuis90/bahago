package guild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	_guild "bahago/internal/guild"
	. "bahago/internal/ui"
	"bahago/internal/routes"
)

// ── Input struct ──────────────────────────────────────────────────────────────

type settingsSignals struct {
	GuildMsgAll      string `json:"guild_msg_all"`
	GuildMsgOfficers string `json:"guild_msg_officers"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		guild, viewerRole, err := h.getGuildAndViewerRole(r.Context(), slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			http.Error(w, "guild not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("guild settings: get guild: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !viewerRole.IsLeader() {
			http.Redirect(w, r, slugURL(routes.GuildViewPath, slug), http.StatusFound)
			return
		}

		perms := _guild.ParseMessagePermissions(guild.Settings)
		KingdomLayout(r, "Settings — "+guild.Name, r.URL.Path, kingdom,
			guildSettingsContent(guild, perms),
		).Render(w)
	}
}

func (h *handler) handleSettingsSave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		input := &settingsSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("guild settings save: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildSettingsAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateSettingsInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(guildSettingsAlert(AlertError(errs...)))
			return
		}

		sse := datastar.NewSSE(w, r)

		if err := h.updateGuildSettings(r.Context(), slug, kingdom.ID, input); err != nil {
			if isSettingsSaveUserError(err) {
				sse.PatchElementGostar(guildSettingsAlert(AlertError(err)))
				return
			}
			log.Printf("guild settings save: %v", err)
			sse.PatchElementGostar(guildSettingsAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := sse.PatchElementGostar(guildSettingsAlert(AlertSuccess("Settings saved."))); err != nil {
			log.Printf("guild settings save: patch: %v", err)
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

var ErrInvalidPermissionValue = errors.New("invalid permission value")
var ErrOnlyLeaderCanChangeSettings = errors.New("only the guild leader can change settings")

func validateSettingsInput(in *settingsSignals) []error {
	var errs []error
	msgAll := _guild.MemberRole(in.GuildMsgAll)
	msgOfficers := _guild.MemberRole(in.GuildMsgOfficers)
	if msgAll.Rank() == 0 || msgOfficers.Rank() == 0 {
		errs = append(errs, ErrInvalidPermissionValue)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// updateGuildSettings loads the guild, enforces leader-only access, marshals
// the permissions JSON, and writes the settings row. Returns sentinels for
// guild-not-found and not-leader cases.
func (h *handler) updateGuildSettings(ctx context.Context, slug string, kingdomID int, input *settingsSignals) error {
	guild, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, kingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return ErrGuildNotFound
	}
	if err != nil {
		return fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.IsLeader() {
		return ErrOnlyLeaderCanChangeSettings
	}

	perms := _guild.MessagePermissions{
		MsgAll:      _guild.MemberRole(input.GuildMsgAll),
		MsgOfficers: _guild.MemberRole(input.GuildMsgOfficers),
	}
	raw, err := json.Marshal(perms)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := h.queries.UpdateGuildSettings(ctx, db.UpdateGuildSettingsParams{
		ID:       guild.ID,
		Settings: raw,
	}); err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	return nil
}

func isSettingsSaveUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) || errors.Is(err, ErrOnlyLeaderCanChangeSettings)
}

// ── Components ────────────────────────────────────────────────────────────────

func guildSettingsContent(guild db.Guild, perms _guild.MessagePermissions) Node {
	roleOptions := []struct {
		Value string
		Label string
	}{
		{string(_guild.RoleMember), "All members"},
		{string(_guild.RoleOfficer), "Officers and leader"},
		{string(_guild.RoleLeader), "Leader only"},
	}

	return Group([]Node{
		Breadcrumb("← Manage "+guild.Name, slugURL(routes.GuildManagePath, guild.Slug)),
		PageHeader("Settings — "+guild.Name),
		guildSettingsAlert(nil),
		Div(Class("card"), Div(Class("card-inner"),
			ds.Signals(map[string]any{
				"guild_msg_all":      string(perms.MsgAll),
				"guild_msg_officers": string(perms.MsgOfficers),
			}),
			Div(Class("card-header"), H2(Class("card-title"), Text("Permissions"))),
			P(Class("card-flavour"), Text("Control who can send guild messages to each recipient group.")),
			Div(Class("settings-fields"),
				Div(Class("field-group"),
					Label(Class("field-label"), For("guild-msg-all"), Text("Who can message all members?")),
					El("select", ID("guild-msg-all"), Class("select"),
						ds.Bind("guild_msg_all"),
						Group(Map(roleOptions, func(opt struct {
							Value string
							Label string
						}) Node {
							return El("option", Value(opt.Value), Text(opt.Label))
						})),
					),
				),
				Div(Class("field-group"),
					Label(Class("field-label"), For("guild-msg-officers"), Text("Who can message officers?")),
					El("select", ID("guild-msg-officers"), Class("select"),
						ds.Bind("guild_msg_officers"),
						Group(Map(roleOptions, func(opt struct {
							Value string
							Label string
						}) Node {
							return El("option", Value(opt.Value), Text(opt.Label))
						})),
					),
				),
			),
			Div(Class("settings-actions"),
				Button(Class("btn btn--primary"),
					ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildSettingsSavePath, guild.Slug))),
					Text("Save Changes")),
				A(Href(slugURL(routes.GuildManagePath, guild.Slug)), Class("btn btn--quiet"), Text("Back to Manage")),
			),
		)),
	})
}

func guildSettingsAlert(inner Node) Node { return AlertContainer("guild-settings-alert", inner) }
