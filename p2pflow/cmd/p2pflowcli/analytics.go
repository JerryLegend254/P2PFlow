package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/JerryLegend254/p2pflow/internal/analytics"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func (app *application) newAnalyticsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "View analytics and intelligence insights",
		Long:  "Commands for viewing file access patterns, predictions, and anomalies detected by the AI intelligence layer",
	}

	cmd.AddCommand(app.newAnalyticsStatsCommand())
	cmd.AddCommand(app.newAnalyticsPredictCommand())
	cmd.AddCommand(app.newAnalyticsPrefetchCommand())
	cmd.AddCommand(app.newAnalyticsAnomaliesCommand())
	cmd.AddCommand(app.newAnalyticsFileCommand())

	return cmd
}

// loadAnalyticsEngine creates and loads an analytics engine, handling missing data gracefully
func loadAnalyticsEngine() (*analytics.AnalyticsEngine, error) {
	config := analytics.DefaultConfig()
	engine, err := analytics.NewAnalyticsEngine(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create analytics engine: %w", err)
	}

	// Load existing data (ignore error if file doesn't exist yet)
	_ = engine.Load()

	return engine, nil
}

func (app *application) newAnalyticsStatsCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show overall analytics statistics",
		Long:  "Display statistics about file access patterns, sync activity, and peer collaboration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load analytics engine
			engine, err := loadAnalyticsEngine()
			if err != nil {
				return err
			}

			// Get statistics
			stats := engine.GetStatistics()

			// Check if we have any data
			if stats.TotalAccesses == 0 {
				fmt.Println("\nNo analytics data found yet.")
				fmt.Println("\nAnalytics are automatically tracked during collaboration sessions.")
				fmt.Println("Start a collaboration session to begin collecting data:")
				fmt.Println("  p2pflow collab serve --file <path>")
				fmt.Println()
				return nil
			}

			if jsonOutput {
				// JSON output
				data, err := stats.ToJSON()
				if err != nil {
					return fmt.Errorf("failed to marshal stats: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			// Pretty print
			cyan := color.New(color.FgCyan).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Printf("\n%s\n", cyan("=== P2PFlow Analytics Statistics ==="))
			fmt.Println()

			fmt.Printf("%s %s\n", cyan("Tracking since:"), stats.StartTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("%s %s\n", cyan("Last updated:"), stats.LastUpdate.Format("2006-01-02 15:04:05"))
			fmt.Println()

			fmt.Printf("%s\n", green("Overall Activity:"))
			fmt.Printf("  Total file accesses: %d\n", stats.TotalAccesses)
			fmt.Printf("  Total changes: %d\n", stats.TotalChanges)
			fmt.Printf("  Unique files tracked: %d\n", stats.UniqueFiles)
			fmt.Printf("  Total bytes changed: %s\n", formatBytes(stats.TotalBytesChanged))
			fmt.Println()

			// Most accessed files
			if len(stats.MostAccessedFiles) > 0 {
				fmt.Printf("%s\n", green("Most Accessed Files:"))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "  File\tAccesses\tChanges\tAvg/Day\tLast Access")
				for i, file := range stats.MostAccessedFiles {
					if i >= 10 {
						break
					}
					lastAccess := file.LastAccess.Format("Jan 02 15:04")
					fmt.Fprintf(w, "  %s\t%d\t%d\t%.1f\t%s\n",
						file.FilePath, file.AccessCount, file.ChangeCount,
						file.AvgAccessFreq, lastAccess)
				}
				w.Flush()
				fmt.Println()
			}

			// Hourly activity pattern
			if len(stats.AccessesByHour) > 0 {
				fmt.Printf("%s\n", green("Access Pattern by Hour:"))
				maxCount := 0
				for _, count := range stats.AccessesByHour {
					if count > maxCount {
						maxCount = count
					}
				}

				for hour := 0; hour < 24; hour++ {
					count := stats.AccessesByHour[hour]
					if count == 0 {
						continue
					}

					barLength := int(float64(count) / float64(maxCount) * 40)
					bar := ""
					for i := 0; i < barLength; i++ {
						bar += "█"
					}

					fmt.Printf("  %02d:00 %s %d\n", hour, yellow(bar), count)
				}
				fmt.Println()
			}

			// Peer activity
			if len(stats.PeerActivity) > 0 {
				fmt.Printf("%s\n", green("Peer Activity:"))
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "  Peer ID\tChanges\tBytes Sent\tLast Activity")

				// Sort peers by activity
				type peerInfo struct {
					id    string
					stats *analytics.PeerStatistics
				}
				peers := make([]peerInfo, 0, len(stats.PeerActivity))
				for id, peerStats := range stats.PeerActivity {
					peers = append(peers, peerInfo{id: id, stats: peerStats})
				}
				sort.Slice(peers, func(i, j int) bool {
					return peers[i].stats.ChangeCount > peers[j].stats.ChangeCount
				})

				for i, peer := range peers {
					if i >= 10 {
						break
					}
					lastActivity := peer.stats.LastActivity.Format("Jan 02 15:04")
					fmt.Fprintf(w, "  %s\t%d\t%s\t%s\n",
						peer.id[:12]+"...", peer.stats.ChangeCount,
						formatBytes(peer.stats.BytesSent), lastActivity)
				}
				w.Flush()
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func (app *application) newAnalyticsPredictCommand() *cobra.Command {
	var currentFile string
	var limit int

	cmd := &cobra.Command{
		Use:   "predict",
		Short: "Predict which files are likely to be accessed next",
		Long:  "Use AI to predict which files you're likely to need based on historical patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load analytics engine
			engine, err := loadAnalyticsEngine()
			if err != nil {
				return err
			}

			// Get predictions
			predictions := engine.PredictNextFiles(currentFile, limit)

			if len(predictions) == 0 {
				fmt.Println("No predictions available. Need more historical data.")
				return nil
			}

			// Pretty print
			cyan := color.New(color.FgCyan).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Printf("\n%s\n", cyan("=== File Access Predictions ==="))
			if currentFile != "" {
				fmt.Printf("Based on current file: %s\n", currentFile)
			}
			fmt.Println()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "File\tConfidence\tReason")

			for _, pred := range predictions {
				confidenceStr := fmt.Sprintf("%.0f%%", pred.Confidence*100)
				if pred.Confidence >= 0.8 {
					confidenceStr = green(confidenceStr)
				} else if pred.Confidence >= 0.6 {
					confidenceStr = yellow(confidenceStr)
				}

				fmt.Fprintf(w, "%s\t%s\t%s\n", pred.FilePath, confidenceStr, pred.Reason)
			}
			w.Flush()
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVarP(&currentFile, "file", "f", "", "Current file to base predictions on")
	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "Maximum number of predictions")

	return cmd
}

func (app *application) newAnalyticsPrefetchCommand() *cobra.Command {
	var currentFiles []string
	var maxSuggestions int

	cmd := &cobra.Command{
		Use:   "prefetch",
		Short: "Get intelligent prefetch suggestions",
		Long:  "Show which files should be prefetched based on predicted access patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load analytics engine
			engine, err := loadAnalyticsEngine()
			if err != nil {
				return err
			}

			// Get prefetch suggestions
			suggestions := engine.GetPrefetchSuggestions(currentFiles, maxSuggestions)

			if len(suggestions) == 0 {
				fmt.Println("No prefetch suggestions available.")
				return nil
			}

			// Pretty print
			cyan := color.New(color.FgCyan).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Printf("\n%s\n", cyan("=== Intelligent Prefetch Suggestions ==="))
			fmt.Println()

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Priority\tFile\tConfidence\tReason")

			for _, sugg := range suggestions {
				priorityStr := fmt.Sprintf("%.0f%%", sugg.Priority*100)
				confidenceStr := fmt.Sprintf("%.0f%%", sugg.Confidence*100)

				if sugg.Priority >= 0.8 {
					priorityStr = green(priorityStr)
				} else if sugg.Priority >= 0.6 {
					priorityStr = yellow(priorityStr)
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					priorityStr, sugg.FilePath, confidenceStr, sugg.Reason)
			}
			w.Flush()
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&currentFiles, "context", "c", []string{}, "Current files for context")
	cmd.Flags().IntVarP(&maxSuggestions, "limit", "l", 10, "Maximum number of suggestions")

	return cmd
}

func (app *application) newAnalyticsAnomaliesCommand() *cobra.Command {
	var jsonOutput bool
	var severityFilter string

	cmd := &cobra.Command{
		Use:   "anomalies",
		Short: "Detect unusual sync patterns and anomalies",
		Long:  "Show detected anomalies in file access patterns, which may indicate issues or security concerns",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load analytics engine
			engine, err := loadAnalyticsEngine()
			if err != nil {
				return err
			}

			// Detect anomalies
			anomalies := engine.DetectAnomalies()

			// Filter by severity if specified
			if severityFilter != "" {
				filtered := make([]analytics.Anomaly, 0)
				for _, anomaly := range anomalies {
					if string(anomaly.Severity) == severityFilter {
						filtered = append(filtered, anomaly)
					}
				}
				anomalies = filtered
			}

			if len(anomalies) == 0 {
				fmt.Println("No anomalies detected. Everything looks normal!")
				return nil
			}

			if jsonOutput {
				data, err := json.MarshalIndent(anomalies, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal anomalies: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			// Pretty print
			cyan := color.New(color.FgCyan).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()
			magenta := color.New(color.FgMagenta).SprintFunc()

			fmt.Printf("\n%s\n", cyan("=== Detected Anomalies ==="))
			fmt.Printf("Found %d anomal%s\n\n", len(anomalies), map[bool]string{true: "y", false: "ies"}[len(anomalies) == 1])

			for i, anomaly := range anomalies {
				// Severity indicator
				severityColor := yellow
				severityIcon := "!"
				switch anomaly.Severity {
				case analytics.SeverityCritical:
					severityColor = red
					severityIcon = "!!"
				case analytics.SeverityHigh:
					severityColor = red
					severityIcon = "!"
				case analytics.SeverityMedium:
					severityColor = yellow
					severityIcon = "!"
				case analytics.SeverityLow:
					severityColor = color.New(color.FgWhite).SprintFunc()
					severityIcon = "i"
				}

				fmt.Printf("%s %s [%s] %s\n",
					severityIcon,
					severityColor(string(anomaly.Severity)),
					magenta(string(anomaly.Type)),
					anomaly.Description)

				if anomaly.FilePath != "" {
					fmt.Printf("   File: %s\n", anomaly.FilePath)
				}
				if anomaly.PeerID != "" {
					fmt.Printf("   Peer: %s\n", anomaly.PeerID)
				}

				fmt.Printf("   Score: %.2f | Time: %s\n",
					anomaly.Score, anomaly.Timestamp.Format("Jan 02 15:04:05"))

				if len(anomaly.Details) > 0 {
					fmt.Printf("   Details: ")
					first := true
					for key, value := range anomaly.Details {
						if !first {
							fmt.Printf(", ")
						}
						fmt.Printf("%s=%v", key, value)
						first = false
					}
					fmt.Println()
				}

				if i < len(anomalies)-1 {
					fmt.Println()
				}
			}
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&severityFilter, "severity", "", "Filter by severity (low, medium, high, critical)")

	return cmd
}

func (app *application) newAnalyticsFileCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "file <path>",
		Short: "Show analytics for a specific file",
		Long:  "Display detailed access patterns and statistics for a specific file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			// Load analytics engine
			engine, err := loadAnalyticsEngine()
			if err != nil {
				return err
			}

			// Get file statistics
			fileStats := engine.GetFileStatistics(filePath)
			if fileStats == nil {
				return fmt.Errorf("no data found for file: %s", filePath)
			}

			if jsonOutput {
				data, err := json.MarshalIndent(fileStats, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal file stats: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			// Pretty print
			cyan := color.New(color.FgCyan).SprintFunc()
			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Printf("\n%s\n", cyan("=== File Analytics ==="))
			fmt.Printf("File: %s\n\n", filePath)

			fmt.Printf("%s\n", green("Access Summary:"))
			fmt.Printf("  Total accesses: %d\n", fileStats.TotalAccesses)
			fmt.Printf("  Reads: %d\n", fileStats.ReadCount)
			fmt.Printf("  Writes: %d\n", fileStats.WriteCount)
			fmt.Printf("  Creates: %d\n", fileStats.CreateCount)
			fmt.Printf("  Syncs: %d\n", fileStats.SyncCount)
			fmt.Printf("  Total bytes: %s\n", formatBytes(fileStats.TotalBytes))
			fmt.Printf("  First access: %s\n", fileStats.FirstAccess.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Last access: %s\n", fileStats.LastAccess.Format("2006-01-02 15:04:05"))
			fmt.Println()

			// Importance score
			importance := engine.GetFileImportance(filePath)
			importanceStr := fmt.Sprintf("%.0f%%", importance*100)
			if importance >= 0.8 {
				importanceStr = green(importanceStr)
			} else if importance >= 0.6 {
				importanceStr = yellow(importanceStr)
			}
			fmt.Printf("%s %s\n", green("Importance Score:"), importanceStr)
			fmt.Println()

			// Hourly pattern
			if len(fileStats.HourlyPattern) > 0 {
				fmt.Printf("%s\n", green("Hourly Access Pattern:"))
				maxCount := 0
				for _, count := range fileStats.HourlyPattern {
					if count > maxCount {
						maxCount = count
					}
				}

				for hour := 0; hour < 24; hour++ {
					count := fileStats.HourlyPattern[hour]
					if count == 0 {
						continue
					}

					barLength := int(float64(count) / float64(maxCount) * 30)
					bar := ""
					for i := 0; i < barLength; i++ {
						bar += "█"
					}

					fmt.Printf("  %02d:00 %s %d\n", hour, yellow(bar), count)
				}
				fmt.Println()
			}

			// Peer access
			if len(fileStats.PeerAccesses) > 0 {
				fmt.Printf("%s\n", green("Peer Access:"))
				for peerID, count := range fileStats.PeerAccesses {
					fmt.Printf("  %s: %d accesses\n", peerID, count)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}
