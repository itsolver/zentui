package cmd

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/johanviberg/zd/internal/permissions"
)

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
	rootCmd.AddCommand(mcpCmd)
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol server for AI agents",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server on stdio",
	Long: "Starts an MCP server that exposes Zendesk operations as tools. " +
		"Communicates over stdin/stdout using the MCP protocol.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ticketSvc, err := newTicketService(cmd)
		if err != nil {
			return err
		}

		searchSvc, err := newSearchService(cmd)
		if err != nil {
			return err
		}

		articleSvc, err := newArticleService(cmd)
		if err != nil {
			return err
		}

		// Fetch user permissions (fail-open if unavailable)
		var perms permissions.Permissions
		userSvc, userErr := newUserService(cmd)
		if userErr == nil {
			user, _ := userSvc.GetMe(cmd.Context())
			perms = permissions.FromUser(user)
		} else {
			perms = permissions.FromUser(nil)
		}

		server := mcp.NewServer(&mcp.Implementation{
			Name:    "zd",
			Version: buildVersion,
		}, nil)

		registerTicketTools(server, ticketSvc, perms)
		registerSearchTools(server, searchSvc)
		registerArticleTools(server, articleSvc)

		return server.Run(cmd.Context(), &mcp.StdioTransport{})
	},
}
