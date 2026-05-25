package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/itsolver/zentui/internal/api"
	"github.com/itsolver/zentui/internal/auth"
	"github.com/itsolver/zentui/internal/cache"
	"github.com/itsolver/zentui/internal/demo"
	"github.com/itsolver/zentui/internal/tui"
	"github.com/itsolver/zentui/pkg/zendesk"
)

func init() {
	tuiCmd.Flags().Int64("view-id", defaultTriageViewID(), "Zendesk view ID to load in the queue")
	tuiCmd.Flags().Int("limit", 20, "Number of tickets to request per page")
	tuiCmd.Flags().String("customer-support-dir", "/Users/angusmclauchlan/Projects/itsolver/customer-support", "Customer-support repo used for local Codex prompt packs")
	tuiCmd.Flags().String("codex-model", "", "Optional Codex model override")
	tuiCmd.Flags().String("codex-reasoning-effort", "", "Optional Codex model reasoning effort")
	tuiCmd.Flags().String("python-bin", "", "Python interpreter for customer-support prompt-pack helpers")
	tuiCmd.Flags().String("work-dir", ".ticket-triage-work", "Local ticket triage working folder")
	rootCmd.AddCommand(tuiCmd)
}

func defaultTriageViewID() int64 {
	const fallback int64 = 7484423111055
	raw := os.Getenv("TICKET_TRIAGE_VIEW_ID")
	if raw == "" {
		return fallback
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return fallback
	}
	return id
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive terminal UI for managing tickets",
	Long:  "Launch an interactive terminal interface for browsing, viewing, and managing Zendesk tickets.",
	RunE: func(cmd *cobra.Command, args []string) error {
		var ticketSvc zendesk.TicketService
		var searchSvc zendesk.SearchService
		var userSvc zendesk.UserService
		var attachmentHTTPClient *http.Client
		if store := demoStoreFromCtx(cmd.Context()); store != nil {
			ticketSvc = demo.NewTicketService(store)
			searchSvc = demo.NewSearchService(store)
			userSvc = demo.NewUserService(store)
		} else {
			client, err := buildClient(cmd)
			if err != nil {
				return err
			}
			ticketSvc = api.NewTicketService(client)
			searchSvc = api.NewSearchService(client)
			userSvc = api.NewUserService(client)
			attachmentHTTPClient = client.HTTPClient
			c := cache.New(60 * time.Second)
			ticketSvc = cache.NewCachedTicketService(ticketSvc, c)
			searchSvc = cache.NewCachedSearchService(searchSvc, c)
		}

		cfg := configFromCtx(cmd.Context())
		profile, _ := cmd.Flags().GetString("profile")
		subdomain := cfg.Subdomain
		if subdomain == "" {
			if creds, _ := auth.ResolveCredentials(profile); creds != nil {
				subdomain = creds.Subdomain
			}
		}

		viewID, _ := cmd.Flags().GetInt64("view-id")
		limit, _ := cmd.Flags().GetInt("limit")
		customerSupportDir, _ := cmd.Flags().GetString("customer-support-dir")
		codexModel, _ := cmd.Flags().GetString("codex-model")
		codexReasoning, _ := cmd.Flags().GetString("codex-reasoning-effort")
		pythonBin, _ := cmd.Flags().GetString("python-bin")
		workDir, _ := cmd.Flags().GetString("work-dir")
		app := tui.NewAppWithOptions(ticketSvc, searchSvc, userSvc, subdomain, buildVersion, tui.AppOptions{
			ViewID:             viewID,
			Limit:              limit,
			CustomerSupportDir: customerSupportDir,
			CodexModel:         codexModel,
			CodexReasoning:     codexReasoning,
			PythonBin:          pythonBin,
			WorkDir:            workDir,
			HTTPClient:         attachmentHTTPClient,
		})
		p := tea.NewProgram(app)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("running TUI: %w", err)
		}
		return nil
	},
}
