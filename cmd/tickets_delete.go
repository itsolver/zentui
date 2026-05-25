package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/itsolver/zentui/internal/config"
)

const deleteConfirmationTTL = 15 * time.Minute

type deleteConfirmation struct {
	TicketID  int64     `json:"ticket_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type deleteConfirmationStore struct {
	Confirmations map[string]deleteConfirmation `json:"confirmations"`
}

func init() {
	ticketsCmd.AddCommand(ticketsDeleteCmd)

	ticketsDeleteCmd.Flags().Bool("dry-run", false, "Preview deletion and return confirmation ID")
	ticketsDeleteCmd.Flags().String("confirm", "", "Execute deletion with confirmation ID from dry-run")
	ticketsDeleteCmd.Flags().Bool("yes", false, "Skip two-step confirmation")
}

var ticketsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ticket ID: %s", args[0])
		}

		perms := ensurePermissions(cmd)
		if !perms.CanDeleteTickets {
			return fmt.Errorf("light agents cannot delete tickets")
		}

		svc, err := newTicketService(cmd)
		if err != nil {
			return err
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		confirmID, _ := cmd.Flags().GetString("confirm")
		yes, _ := cmd.Flags().GetBool("yes")
		globalYes, _ := cmd.Root().Flags().GetBool("yes")
		yes = yes || globalYes

		if dryRun {
			result, err := svc.Get(cmd.Context(), id, nil)
			if err != nil {
				return err
			}

			confirmation, err := generateConfirmationID()
			if err != nil {
				return fmt.Errorf("generating confirmation ID: %w", err)
			}
			if err := saveDeleteConfirmation(confirmation, id, time.Now()); err != nil {
				return fmt.Errorf("saving confirmation ID: %w", err)
			}

			formatter := formatterFromCtx(cmd.Context())
			dryRunResult := map[string]interface{}{
				"action":          "delete",
				"ticket_id":       result.Ticket.ID,
				"subject":         result.Ticket.Subject,
				"status":          result.Ticket.Status,
				"confirmation_id": confirmation,
				"message":         fmt.Sprintf("Run 'zentui tickets delete %d --confirm %s' to execute", id, confirmation),
			}
			return formatter.Format(os.Stdout, dryRunResult)
		}

		if confirmID != "" {
			if err := consumeDeleteConfirmation(confirmID, id, time.Now()); err != nil {
				return err
			}
		} else if !yes {
			return fmt.Errorf("deletion requires --yes, --dry-run/--confirm, or interactive confirmation")
		}

		if err := svc.Delete(cmd.Context(), id); err != nil {
			return err
		}

		formatter := formatterFromCtx(cmd.Context())
		result := map[string]interface{}{
			"deleted":   true,
			"ticket_id": id,
		}
		return formatter.Format(os.Stdout, result)
	},
}

func generateConfirmationID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func deleteConfirmationPath() string {
	return filepath.Join(config.ConfigDir(), "delete-confirmations.json")
}

func saveDeleteConfirmation(confirmation string, ticketID int64, now time.Time) error {
	store, err := loadDeleteConfirmationStore(now)
	if err != nil {
		return err
	}
	if store.Confirmations == nil {
		store.Confirmations = make(map[string]deleteConfirmation)
	}
	store.Confirmations[confirmation] = deleteConfirmation{
		TicketID:  ticketID,
		ExpiresAt: now.Add(deleteConfirmationTTL),
	}
	return writeDeleteConfirmationStore(store)
}

func consumeDeleteConfirmation(confirmation string, ticketID int64, now time.Time) error {
	store, err := loadDeleteConfirmationStore(now)
	if err != nil {
		return err
	}
	pending, ok := store.Confirmations[confirmation]
	if !ok {
		return fmt.Errorf("invalid or expired confirmation ID: %s (run --dry-run first)", confirmation)
	}
	if pending.TicketID != ticketID {
		return fmt.Errorf("confirmation ID was for ticket %d, not %d", pending.TicketID, ticketID)
	}
	delete(store.Confirmations, confirmation)
	return writeDeleteConfirmationStore(store)
}

func loadDeleteConfirmationStore(now time.Time) (*deleteConfirmationStore, error) {
	path := deleteConfirmationPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &deleteConfirmationStore{Confirmations: map[string]deleteConfirmation{}}, nil
		}
		return nil, fmt.Errorf("reading delete confirmations: %w", err)
	}
	var store deleteConfirmationStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parsing delete confirmations: %w", err)
	}
	if store.Confirmations == nil {
		store.Confirmations = map[string]deleteConfirmation{}
	}
	for id, pending := range store.Confirmations {
		if !pending.ExpiresAt.After(now) {
			delete(store.Confirmations, id)
		}
	}
	return &store, nil
}

func writeDeleteConfirmationStore(store *deleteConfirmationStore) error {
	path := deleteConfirmationPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling delete confirmations: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".delete-confirmations-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
