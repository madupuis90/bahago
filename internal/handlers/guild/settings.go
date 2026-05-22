package guild

import (
	"encoding/json"
	"errors"
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

		guild, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
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

		sse := datastar.NewSSE(w, r)

		guild, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildSettingsAlert(AlertError(errors.New("guild not found"))))
			return
		}
		if err != nil {
			log.Printf("guild settings save: get guild: %v", err)
			sse.PatchElementGostar(guildSettingsAlert(AlertError(errors.New("internal error"))))
			return
		}
		if !viewerRole.IsLeader() {
			sse.PatchElementGostar(guildSettingsAlert(AlertError(errors.New("only the guild leader can change settings"))))
			return
		}

		msgAll := _guild.MemberRole(input.GuildMsgAll)
		msgOfficers := _guild.MemberRole(input.GuildMsgOfficers)
		if msgAll.Rank() == 0 || msgOfficers.Rank() == 0 {
			sse.PatchElementGostar(guildSettingsAlert(AlertError(errors.New("invalid permission value"))))
			return
		}

		perms := _guild.MessagePermissions{
			MsgAll:      msgAll,
			MsgOfficers: msgOfficers,
		}
		raw, err := json.Marshal(perms)
		if err != nil {
			log.Printf("guild settings save: marshal: %v", err)
			sse.PatchElementGostar(guildSettingsAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := h.queries.UpdateGuildSettings(r.Context(), db.UpdateGuildSettingsParams{
			ID:       guild.ID,
			Settings: raw,
		}); err != nil {
			log.Printf("guild settings save: update: %v", err)
			sse.PatchElementGostar(guildSettingsAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := sse.PatchElementGostar(guildSettingsAlert(AlertSuccess("Settings saved."))); err != nil {
			log.Printf("guild settings save: patch: %v", err)
		}
	}
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
		H1(Class("page-title"), Text("Guild Settings — "+guild.Name)),
		guildSettingsAlert(nil),
		Div(Class("guild-settings panel"),
			ds.Signals(map[string]any{
				"guild_msg_all":      string(perms.MsgAll),
				"guild_msg_officers": string(perms.MsgOfficers),
			}),
			P(Class("panel-title"), Text("Permissions")),
			P(Class("guild-settings-hint"), Text("Control who can send guild messages to each recipient group.")),
			Div(Class("guild-settings-fields"),
				Label(For("guild-msg-all"), Text("Who can message all members?")),
				Select(
					ID("guild-msg-all"),
					ds.Bind("guild_msg_all"),
					Group(Map(roleOptions, func(opt struct {
						Value string
						Label string
					}) Node {
						return Option(Value(opt.Value), Text(opt.Label))
					})),
				),

				Label(For("guild-msg-officers"), Text("Who can message officers?")),
				Select(
					ID("guild-msg-officers"),
					ds.Bind("guild_msg_officers"),
					Group(Map(roleOptions, func(opt struct {
						Value string
						Label string
					}) Node {
						return Option(Value(opt.Value), Text(opt.Label))
					})),
				),
			),
			Div(Class("guild-settings-actions"),
				Button(
					Class("btn"),
					ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildSettingsSavePath, guild.Slug))),
					Text("Save"),
				),
				A(Href(slugURL(routes.GuildManagePath, guild.Slug)), Class("btn btn-text"), Text("Back to Manage")),
			),
		),
	})
}

func guildSettingsAlert(inner Node) Node { return AlertContainer("guild-settings-alert", inner) }
