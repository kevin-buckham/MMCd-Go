package cli

import (
	"fmt"
	"os"

	"github.com/kbuckham/mmcd/internal/version"
	"github.com/spf13/cobra"
)

var (
	cfgPort  string
	cfgBaud  int
	cfgUnits string
)

// rootCmd is the base command when called without subcommands.
var rootCmd = &cobra.Command{
	Use:     "mmcd",
	Short:   "MMCD Datalogger — 1G DSM ECU diagnostic and datalogging tool",
	Version: version.FullVersion(),
	Long: fmt.Sprintf(`%s v%s
%s

Developed by %s
%s

Use subcommands for headless CLI operation (log, dtc, test, review, import, sensors).`,
		version.Name, version.Version, version.Description,
		version.Developers, version.Copyright),
}

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Show application information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s v%s\n", version.Name, version.FullVersion())
		fmt.Println()
		fmt.Println(version.Description)
		fmt.Println()
		fmt.Printf("Developers:  %s\n", version.Developers)
		fmt.Printf("License:     %s\n", version.License)
		fmt.Println(version.Copyright)
		fmt.Printf("Source:      %s\n", version.URL)
		fmt.Printf("Git hash:    %s\n", version.GitHash)
		fmt.Printf("Built:       %s\n", version.BuildTime)
		fmt.Println()
		fmt.Println(version.Attribution)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPort, "port", "p", "", "Serial port (e.g. /dev/ttyUSB0, COM3)")
	rootCmd.PersistentFlags().IntVarP(&cfgBaud, "baud", "b", 1953, "Serial baud rate")
	rootCmd.PersistentFlags().StringVarP(&cfgUnits, "units", "u", "metric", "Unit system: metric, imperial, raw")
	rootCmd.AddCommand(aboutCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// RootCmd returns the root cobra command (for Wails integration).
func RootCmd() *cobra.Command {
	return rootCmd
}
