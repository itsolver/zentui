package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	tuiCmd.Flags().Int64("view-id", defaultTriageViewID(), "Zendesk view ID to load in the queue (default from TICKET_TRIAGE_VIEW_ID; 0 lists tickets)")
	tuiCmd.Flags().Int("limit", 20, "Number of tickets to request per page")
	tuiCmd.Flags().String("customer-support-dir", "", "Customer-support repo used for local Codex prompt packs (defaults to ZENTUI_CUSTOMER_SUPPORT_DIR or a sibling customer-support checkout when present)")
	tuiCmd.Flags().String("codex-model", "", "Optional Codex model override")
	tuiCmd.Flags().String("codex-reasoning-effort", "", "Optional Codex model reasoning effort")
	tuiCmd.Flags().String("python-bin", "", "Python interpreter for customer-support prompt-pack helpers")
	tuiCmd.Flags().String("work-dir", ".ticket-triage-work", "Local ticket triage working folder")
	rootCmd.AddCommand(tuiCmd)
}

func defaultTriageViewID() int64 {
	raw := strings.TrimSpace(os.Getenv("TICKET_TRIAGE_VIEW_ID"))
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func defaultCustomerSupportDir() string {
	if raw := strings.TrimSpace(os.Getenv("ZENTUI_CUSTOMER_SUPPORT_DIR")); raw != "" {
		return raw
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for _, candidate := range []string{
		filepath.Join(wd, "..", "customer-support"),
		filepath.Join(wd, "customer-support"),
	} {
		if localTriageHelperExists(candidate) {
			return candidate
		}
	}
	return ""
}

func localTriageHelperExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "scripts", "local_triage_codex.py"))
	return err == nil && !info.IsDir()
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
		var untrustedAttachmentHTTPClient *http.Client
		var creds *auth.ProfileCredentials
		profile, _ := cmd.Flags().GetString("profile")
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
			untrustedAttachmentHTTPClient = nonAuthAttachmentHTTPClient(client.HTTPClient)
			c := cache.New(60 * time.Second)
			ticketSvc = cache.NewCachedTicketService(ticketSvc, c)
			searchSvc = cache.NewCachedSearchService(searchSvc, c)
			creds, _ = auth.ResolveCredentials(profile)
		}

		cfg := configFromCtx(cmd.Context())
		subdomain := cfg.Subdomain
		if subdomain == "" {
			if creds == nil {
				creds, _ = auth.ResolveCredentials(profile)
			}
			if creds != nil {
				subdomain = creds.Subdomain
			}
		}
		promptPackEnv := zendeskPromptPackEnv(subdomain, creds)
		var trustedAttachmentHosts []string
		if subdomain != "" {
			trustedAttachmentHosts = zendeskAttachmentHosts(subdomain)
		}

		viewID, _ := cmd.Flags().GetInt64("view-id")
		limit, _ := cmd.Flags().GetInt("limit")
		customerSupportDir, _ := cmd.Flags().GetString("customer-support-dir")
		if strings.TrimSpace(customerSupportDir) == "" {
			customerSupportDir = defaultCustomerSupportDir()
		}
		codexModel, _ := cmd.Flags().GetString("codex-model")
		codexReasoning, _ := cmd.Flags().GetString("codex-reasoning-effort")
		pythonBin, _ := cmd.Flags().GetString("python-bin")
		workDir, _ := cmd.Flags().GetString("work-dir")
		app := tui.NewAppWithOptions(ticketSvc, searchSvc, userSvc, subdomain, buildVersion, tui.AppOptions{
			ViewID:              viewID,
			Limit:               limit,
			CustomerSupportDir:  customerSupportDir,
			CodexModel:          codexModel,
			CodexReasoning:      codexReasoning,
			PythonBin:           pythonBin,
			WorkDir:             workDir,
			PromptPackEnv:       promptPackEnv,
			HTTPClient:          attachmentHTTPClient,
			UntrustedHTTPClient: untrustedAttachmentHTTPClient,
			TrustedHosts:        trustedAttachmentHosts,
		})
		p := tea.NewProgram(app)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("running TUI: %w", err)
		}
		return nil
	},
}

func zendeskPromptPackEnv(subdomain string, creds *auth.ProfileCredentials) []string {
	if creds == nil || creds.Method != "token" {
		return nil
	}

	env := make([]string, 0, 3)
	if subdomain != "" {
		env = append(env, "ZENDESK_SUBDOMAIN="+subdomain)
	}
	if creds.Email != "" {
		env = append(env, "ZENDESK_EMAIL="+creds.Email)
	}
	if creds.APIToken != "" {
		env = append(env, "ZENDESK_API_TOKEN="+creds.APIToken)
	}
	return env
}

func zendeskAttachmentHosts(subdomain string) []string {
	return []string{
		subdomain + ".zendesk.com",
		".zdusercontent.com",
		".zendeskusercontent.com",
	}
}

func nonAuthAttachmentHTTPClient(source *http.Client) *http.Client {
	timeout := 30 * time.Second
	if source != nil && source.Timeout > 0 {
		timeout = source.Timeout
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Transport: &api.RetryTransport{Base: base, MaxRetries: 3},
		Timeout:   timeout,
	}
}
