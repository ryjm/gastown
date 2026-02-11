package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/mail"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/witness"
	"github.com/steveyegge/gastown/internal/workspace"
)

var (
	witnessInboxDryRun  bool
	witnessInboxVerbose bool
	witnessInboxJSON    bool
)

var witnessProcessInboxCmd = &cobra.Command{
	Use:   "process-inbox <rig>",
	Short: "Process witness inbox messages using protocol handlers",
	Long: `Process the witness's inbox, dispatching each message to the
appropriate handler based on the witness protocol.

Handles these message types:
  POLECAT_DONE       - Polecat signals work completion (nuke or create cleanup wisp)
  LIFECYCLE:Shutdown  - Daemon-triggered polecat shutdown
  HELP:              - Polecat requesting intervention (assess or escalate)
  MERGED             - Refinery confirms branch merged (nuke polecat)
  MERGE_FAILED       - Refinery reports merge failure (notify polecat)
  SWARM_START        - Mayor initiating batch work (create tracking wisp)

Messages are archived (marked read) after successful handling.
Unknown message types are skipped.

Examples:
  gt witness process-inbox gastown
  gt witness process-inbox gastown --dry-run
  gt witness process-inbox gastown --json`,
	Args: cobra.ExactArgs(1),
	RunE: runWitnessProcessInbox,
}

// WitnessInboxResult is the JSON output for a single processed message.
type WitnessInboxResult struct {
	MessageID    string `json:"message_id"`
	From         string `json:"from"`
	Subject      string `json:"subject"`
	ProtocolType string `json:"protocol_type"`
	Handled      bool   `json:"handled"`
	Action       string `json:"action,omitempty"`
	WispCreated  string `json:"wisp_created,omitempty"`
	MailSent     string `json:"mail_sent,omitempty"`
	Error        string `json:"error,omitempty"`
}

func init() {
	witnessProcessInboxCmd.Flags().BoolVar(&witnessInboxDryRun, "dry-run", false, "Show what would be processed without taking action")
	witnessProcessInboxCmd.Flags().BoolVarP(&witnessInboxVerbose, "verbose", "v", false, "Show detailed processing info")
	witnessProcessInboxCmd.Flags().BoolVar(&witnessInboxJSON, "json", false, "Output as JSON")

	witnessCmd.AddCommand(witnessProcessInboxCmd)
}

func runWitnessProcessInbox(cmd *cobra.Command, args []string) error {
	rigName := args[0]

	townRoot, err := workspace.FindFromCwdOrError()
	if err != nil {
		return fmt.Errorf("not in a Gas Town workspace: %w", err)
	}

	// Get witness's mailbox
	witnessAddr := fmt.Sprintf("%s/witness", rigName)
	router := mail.NewRouter(townRoot)
	mailbox, err := router.GetMailbox(witnessAddr)
	if err != nil {
		return fmt.Errorf("getting witness mailbox: %w", err)
	}

	// Get unread messages
	messages, err := mailbox.ListUnread()
	if err != nil {
		return fmt.Errorf("listing unread messages: %w", err)
	}

	if len(messages) == 0 {
		if witnessInboxJSON {
			fmt.Println("[]")
		} else {
			fmt.Printf("%s No pending messages in witness inbox\n", style.Dim.Render("○"))
		}
		return nil
	}

	if !witnessInboxJSON {
		fmt.Printf("%s Processing %d message(s) in %s witness inbox\n",
			style.Bold.Render("●"), len(messages), rigName)
	}

	var results []WitnessInboxResult

	for _, msg := range messages {
		// Classify the message
		protoType := witness.ClassifyMessage(msg.Subject)

		result := WitnessInboxResult{
			MessageID:    msg.ID,
			From:         msg.From,
			Subject:      msg.Subject,
			ProtocolType: string(protoType),
		}

		if witnessInboxDryRun {
			result.Action = fmt.Sprintf("would handle as %s", protoType)
			result.Handled = protoType != witness.ProtoUnknown
			results = append(results, result)
			continue
		}

		// Dispatch to the appropriate handler
		var handlerResult *witness.HandlerResult

		switch protoType {
		case witness.ProtoPolecatDone:
			handlerResult = witness.HandlePolecatDone(townRoot, rigName, msg, router)

		case witness.ProtoLifecycleShutdown:
			handlerResult = witness.HandleLifecycleShutdown(townRoot, rigName, msg)

		case witness.ProtoHelp:
			handlerResult = witness.HandleHelp(townRoot, rigName, msg, router)

		case witness.ProtoMerged:
			handlerResult = witness.HandleMerged(townRoot, rigName, msg)

		case witness.ProtoMergeFailed:
			handlerResult = witness.HandleMergeFailed(townRoot, rigName, msg, router)

		case witness.ProtoSwarmStart:
			handlerResult = witness.HandleSwarmStart(townRoot, msg)

		case witness.ProtoHandoff:
			// Handoff messages are informational - just archive
			result.Handled = true
			result.Action = "archived handoff message"

		default:
			result.Action = "unknown message type, skipped"
		}

		// Convert handler result
		if handlerResult != nil {
			result.Handled = handlerResult.Handled
			result.Action = handlerResult.Action
			result.WispCreated = handlerResult.WispCreated
			result.MailSent = handlerResult.MailSent
			if handlerResult.Error != nil {
				result.Error = handlerResult.Error.Error()
			}
		}

		// Archive handled messages
		if result.Handled {
			if archiveErr := mailbox.MarkRead(msg.ID); archiveErr != nil {
				// Non-fatal: message was handled, just not archived
				if result.Error == "" {
					result.Error = fmt.Sprintf("archive failed: %v", archiveErr)
				}
			}
		}

		results = append(results, result)
	}

	// Output results
	if witnessInboxJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	// Human-readable output
	for _, r := range results {
		if r.Error != "" {
			fmt.Printf("  %s [%s] %s: %s\n",
				style.Error.Render("✗"),
				r.ProtocolType,
				r.Subject,
				r.Error)
		} else if r.Handled {
			fmt.Printf("  %s [%s] %s\n",
				style.Bold.Render("✓"),
				r.ProtocolType,
				r.Action)
		} else {
			fmt.Printf("  %s [%s] %s\n",
				style.Dim.Render("○"),
				r.ProtocolType,
				r.Action)
		}

		if witnessInboxVerbose {
			fmt.Printf("      From: %s\n", r.From)
			fmt.Printf("      Subject: %s\n", r.Subject)
			if r.WispCreated != "" {
				fmt.Printf("      Wisp: %s\n", r.WispCreated)
			}
			if r.MailSent != "" {
				fmt.Printf("      Mail: %s\n", r.MailSent)
			}
		}
	}

	// Summary
	handled := 0
	errors := 0
	for _, r := range results {
		if r.Handled {
			handled++
		}
		if r.Error != "" {
			errors++
		}
	}

	fmt.Println()
	if witnessInboxDryRun {
		fmt.Printf("%s Dry run: would process %d/%d messages\n",
			style.Dim.Render("○"), handled, len(results))
	} else {
		fmt.Printf("%s Processed %d/%d messages",
			style.Bold.Render("✓"), handled, len(results))
		if errors > 0 {
			fmt.Printf(" (%d errors)", errors)
		}
		fmt.Println()
	}

	return nil
}
