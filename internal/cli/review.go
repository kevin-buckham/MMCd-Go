package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var reviewFile string

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review a saved CSV log file in the terminal",
	RunE: func(cmd *cobra.Command, args []string) error {
		if reviewFile == "" {
			return fmt.Errorf("--file is required")
		}

		f, err := os.Open(reviewFile)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()

		reader := csv.NewReader(f)

		// Read header
		header, err := reader.Read()
		if err != nil {
			return fmt.Errorf("failed to read CSV header: %w", err)
		}

		fmt.Printf("Log file: %s\n", reviewFile)
		fmt.Printf("Columns: %d\n\n", len(header))

		// Print header
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for i, h := range header {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, h)
		}
		fmt.Fprintln(w)

		// Print separator
		for i := range header {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, "---")
		}
		fmt.Fprintln(w)

		// Print rows (limit to first 50 for terminal display)
		rowCount := 0
		for {
			row, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("CSV read error at row %d: %w", rowCount+1, err)
			}

			rowCount++
			if rowCount > 50 {
				continue // count but don't print
			}

			for i, val := range row {
				if i > 0 {
					fmt.Fprint(w, "\t")
				}
				fmt.Fprint(w, val)
			}
			fmt.Fprintln(w)
		}
		w.Flush()

		if rowCount > 50 {
			fmt.Printf("\n... showing first 50 of %d rows\n", rowCount)
		} else {
			fmt.Printf("\n%d rows total\n", rowCount)
		}

		return nil
	},
}

func init() {
	reviewCmd.Flags().StringVarP(&reviewFile, "file", "f", "", "CSV log file to review")
	rootCmd.AddCommand(reviewCmd)
}
