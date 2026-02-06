package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kbuckham/mmcd/internal/logger"
	"github.com/kbuckham/mmcd/internal/sensor"
	"github.com/spf13/cobra"
)

var (
	importFile   string
	importOutput string
	importFormat string
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import an old MMCd PalmOS PDB log file and convert to CSV or .mmcd",
	Long: `Reads a PalmOS PDB file from the original MMCd datalogger and converts
it to CSV (human-readable) or .mmcd (native binary, for replay).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if importFile == "" {
			return fmt.Errorf("--file is required")
		}

		units := sensor.ParseUnitSystem(cfgUnits)
		defs := sensor.DefaultDefinitions()

		fmt.Printf("Parsing PDB file: %s\n", importFile)
		pdbLog, err := logger.ParsePDB(importFile)
		if err != nil {
			return fmt.Errorf("failed to parse PDB: %w", err)
		}

		fmt.Printf("Log name: %s\n", pdbLog.Name)
		fmt.Printf("Samples:  %d\n", len(pdbLog.Samples))

		if len(pdbLog.Samples) == 0 {
			fmt.Println("No samples found in PDB file.")
			return nil
		}

		// Show time range
		first := pdbLog.Samples[0].Time
		last := pdbLog.Samples[len(pdbLog.Samples)-1].Time
		fmt.Printf("Time range: %s to %s (%.1fs)\n",
			first.Format("2006-01-02 15:04:05"),
			last.Format("2006-01-02 15:04:05"),
			last.Sub(first).Seconds())

		// Show which sensors have data
		var presentMask uint32
		for _, s := range pdbLog.Samples {
			presentMask |= s.DataPresent
		}
		fmt.Printf("Sensors present: ")
		for i := 0; i < sensor.MaxSensors; i++ {
			if presentMask&(1<<uint(i)) != 0 && defs[i].Exists {
				fmt.Printf("%s ", defs[i].Slug)
			}
		}
		fmt.Println()

		// Auto-generate output filename if not specified
		if importOutput == "" {
			base := strings.TrimSuffix(filepath.Base(importFile), filepath.Ext(importFile))
			if importFormat == "mmcd" {
				importOutput = base + ".mmcd"
			} else {
				importOutput = base + ".csv"
			}
		}

		if importFormat == "mmcd" {
			// Convert to native binary format
			var indices []int
			for i := 0; i < sensor.MaxSensors; i++ {
				if presentMask&(1<<uint(i)) != 0 {
					indices = append(indices, i)
				}
			}

			writer, err := logger.NewBinaryWriter(importOutput, indices, units)
			if err != nil {
				return err
			}
			for _, s := range pdbLog.Samples {
				s.ComputeDerivatives(defs)
				if err := writer.WriteSample(s); err != nil {
					writer.Close()
					return err
				}
			}
			writer.Close()
			fmt.Printf("Written %d samples to: %s (binary)\n", writer.Count(), importOutput)
		} else {
			// Convert to CSV
			if err := logger.PDBToCSV(pdbLog, importOutput, defs, units); err != nil {
				return err
			}
			fmt.Printf("Written %d samples to: %s (CSV)\n", len(pdbLog.Samples), importOutput)
		}

		return nil
	},
}

func init() {
	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "PDB file to import")
	importCmd.Flags().StringVarP(&importOutput, "output", "o", "", "Output file (auto-generated if empty)")
	importCmd.Flags().StringVar(&importFormat, "format", "csv", "Output format: csv or mmcd")
	rootCmd.AddCommand(importCmd)
}
