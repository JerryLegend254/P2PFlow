package main

import (
	"fmt"

	"github.com/JerryLegend254/p2pflow/internal/modes"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func (app *application) newModesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modes",
		Short: "List available collaboration modes",
		Long:  "Display all available collaboration modes with their descriptions and configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cyan := color.New(color.FgCyan).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()
			bold := color.New(color.Bold).SprintFunc()

			fmt.Printf("\n%s\n\n", bold("Available Collaboration Modes"))

			availableModes := modes.GetAvailableModes()

			for _, mode := range availableModes {
				config, _ := modes.GetModeConfig(mode)
				description := modes.GetModeDescription(mode)

				fmt.Printf("%s %s\n", cyan("●"), bold(string(mode)))
				fmt.Printf("  %s\n", description)
				fmt.Printf("\n")
				fmt.Printf("  %s:\n", yellow("Configuration"))
				fmt.Printf("    • Realtime Sync:     %s\n", formatBool(config.RealtimeSync))
				fmt.Printf("    • Auto Sync:         %s\n", formatBool(config.AutoSync))
				if config.SyncInterval > 0 {
					fmt.Printf("    • Sync Interval:     %s\n", config.SyncInterval)
				}
				fmt.Printf("    • Can Send Changes:  %s\n", formatBool(config.CanSendChanges))
				fmt.Printf("    • Can Receive:       %s\n", formatBool(config.CanReceiveChanges))
				fmt.Printf("    • Read Only:         %s\n", formatBool(config.ReadOnly))
				fmt.Printf("    • Conflict Strategy: %s\n", config.ConflictStrategy)
				fmt.Printf("    • Bandwidth Profile: %s\n", config.BandwidthProfile)
				fmt.Printf("    • Notifications:     %s\n", config.Notifications)

				if config.RequireApproval {
					fmt.Printf("    • %s\n", green("Requires approval for changes"))
				}
				if config.IsLeader {
					fmt.Printf("    • %s\n", green("Leader mode - changes take precedence"))
				}
				if config.Mode == modes.SelectiveMode {
					fmt.Printf("    • %s\n", yellow("Selective sync - specify paths with --selective-paths"))
				}

				fmt.Printf("\n")
			}

			fmt.Printf("%s\n", bold("Usage Examples:"))
			fmt.Printf("  %s\n", cyan("# Start in realtime mode (default)"))
			fmt.Printf("  p2pflow collab serve /path/to/dir --mode realtime\n\n")
			fmt.Printf("  %s\n", cyan("# Join in observer mode (read-only)"))
			fmt.Printf("  p2pflow collab join session-id --mode observer\n\n")
			fmt.Printf("  %s\n", cyan("# Start with batch mode for reduced bandwidth"))
			fmt.Printf("  p2pflow collab serve /path/to/dir --mode batch\n\n")
			fmt.Printf("  %s\n", cyan("# Leader-follower workflow"))
			fmt.Printf("  p2pflow collab serve /path/to/dir --mode leader\n")
			fmt.Printf("  p2pflow collab join session-id --mode follower\n\n")
			fmt.Printf("  %s\n", cyan("# CRDT-based conflict-free collaboration"))
			fmt.Printf("  p2pflow collab-crdt serve /path/to/dir --mode conflict-free\n\n")

			return nil
		},
	}

	return cmd
}

func formatBool(b bool) string {
	if b {
		green := color.New(color.FgGreen).SprintFunc()
		return green("Yes")
	}
	red := color.New(color.FgRed).SprintFunc()
	return red("No")
}
