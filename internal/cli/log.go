package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/kbuckham/mmcd/internal/logger"
	"github.com/kbuckham/mmcd/internal/protocol"
	"github.com/kbuckham/mmcd/internal/sensor"
	"github.com/spf13/cobra"
)

var (
	logSensors string
	logOutput  string
	logDisplay bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Start datalogging to CSV with optional terminal display",
	Long: `Connects to the ECU via serial port and continuously polls selected sensors.
Data is written to a CSV file and optionally displayed in the terminal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgPort == "" {
			return fmt.Errorf("--port is required (e.g. /dev/ttyUSB0, COM3)")
		}

		units := sensor.ParseUnitSystem(cfgUnits)
		defs := sensor.DefaultDefinitions()

		// Determine which sensors to poll
		var indices []int
		if logSensors == "" || strings.ToLower(logSensors) == "all" {
			indices = sensor.AllPollableIndices(defs)
		} else {
			slugs := strings.Split(strings.ToUpper(logSensors), ",")
			var notFound []string
			indices, notFound = sensor.SlugsToIndices(defs, slugs)
			if len(notFound) > 0 {
				fmt.Fprintf(os.Stderr, "Warning: unknown sensors: %s\n", strings.Join(notFound, ", "))
			}
		}

		if len(indices) == 0 {
			return fmt.Errorf("no valid sensors selected")
		}

		// Also include computed sensors (INJD) if their dependencies are present
		hasRPM := false
		hasINJP := false
		for _, idx := range indices {
			if defs[idx].Slug == "RPM" {
				hasRPM = true
			}
			if defs[idx].Slug == "INJP" {
				hasINJP = true
			}
		}
		if hasRPM && hasINJP {
			injdIdx, _ := sensor.FindBySlug(defs, "INJD")
			if injdIdx >= 0 {
				indices = append(indices, injdIdx)
			}
		}

		fmt.Printf("MMCD Datalogger\n")
		fmt.Printf("Port: %s @ %d baud\n", cfgPort, cfgBaud)
		fmt.Printf("Sensors: %d selected\n", len(indices))
		for _, idx := range indices {
			fmt.Printf("  [%d] %s - %s\n", idx, defs[idx].Slug, defs[idx].Description)
		}

		// Open serial connection
		conn := protocol.NewSerialConn(cfgPort, cfgBaud)
		if err := conn.Open(); err != nil {
			return fmt.Errorf("failed to open serial port: %w", err)
		}
		defer conn.Close()

		ecu := protocol.NewECU(conn, defs)
		lg := logger.New(ecu, defs, indices, units)

		// Set up CSV writer if output file specified
		var csvWriter *logger.CSVWriter
		if logOutput != "" {
			var err error
			csvWriter, err = logger.NewCSVWriter(logOutput, defs, indices, units)
			if err != nil {
				return fmt.Errorf("failed to create CSV file: %w", err)
			}
			defer csvWriter.Close()
			fmt.Printf("Logging to: %s\n", logOutput)
		}

		// Register callbacks
		sampleCount := 0
		startTime := time.Now()

		lg.OnSample(func(sample sensor.Sample) {
			sampleCount++

			// Write to CSV
			if csvWriter != nil {
				if err := csvWriter.WriteSample(sample); err != nil {
					slog.Error("CSV write error", "error", err)
				}
			}

			// Display in terminal
			if logDisplay && sampleCount%5 == 0 {
				elapsed := time.Since(startTime).Seconds()
				hz := float64(sampleCount) / elapsed

				// Clear screen and print values
				fmt.Print("\033[H\033[2J")
				fmt.Printf("MMCD Datalogger — %.1f Hz — %d samples", hz, sampleCount)
				if csvWriter != nil {
					fmt.Printf(" — logging to %s", logOutput)
				}
				fmt.Println()
				fmt.Println(strings.Repeat("─", 60))

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, idx := range indices {
					if !defs[idx].Exists || !sample.HasData(idx) {
						continue
					}
					formatted := defs[idx].Format(sample.RawData[idx], units)
					fmt.Fprintf(w, "%s\t%s\t(raw: %d)\n", defs[idx].Slug, formatted, sample.RawData[idx])
				}
				w.Flush()
				fmt.Println(strings.Repeat("─", 60))
				fmt.Println("Press Ctrl+C to stop")
			}
		})

		// Start logging
		if err := lg.Start(); err != nil {
			return fmt.Errorf("failed to start logger: %w", err)
		}

		// Wait for interrupt
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		fmt.Println("\nStopping...")
		lg.Stop()

		elapsed := time.Since(startTime)
		fmt.Printf("Collected %d samples in %s (%.1f Hz)\n",
			sampleCount, elapsed.Round(time.Millisecond), float64(sampleCount)/elapsed.Seconds())

		if csvWriter != nil {
			fmt.Printf("Saved to: %s (%d rows)\n", logOutput, csvWriter.Count())
		}

		return nil
	},
}

func init() {
	logCmd.Flags().StringVarP(&logSensors, "sensors", "s", "", "Sensor slugs to poll (comma-separated, or 'all')")
	logCmd.Flags().StringVarP(&logOutput, "output", "o", "", "Output CSV file path")
	logCmd.Flags().BoolVarP(&logDisplay, "display", "d", true, "Show live values in terminal")
	rootCmd.AddCommand(logCmd)
}
