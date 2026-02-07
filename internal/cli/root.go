package cli

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/kbuckham/mmcd/internal/version"
	"github.com/spf13/cobra"
)

var (
	cfgPort    string
	cfgBaud    int
	cfgUnits   string
	cfgVerbose bool
	cfgLogFile string
	cfgYes     bool
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
	rootCmd.PersistentFlags().BoolVarP(&cfgVerbose, "verbose", "v", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVar(&cfgLogFile, "log-file", "", "Write log output to file")
	rootCmd.PersistentFlags().BoolVar(&cfgYes, "yes", false, "Skip confirmation prompts")
	rootCmd.AddCommand(aboutCmd)

	cobra.OnInitialize(initLogging)
}

func initLogging() {
	level := slog.LevelInfo
	if cfgVerbose {
		level = slog.LevelDebug
	}

	var w io.Writer = os.Stderr
	if cfgLogFile != "" {
		f, err := os.OpenFile(cfgLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open log file %s: %v\n", cfgLogFile, err)
		} else {
			w = io.MultiWriter(os.Stderr, f)
		}
	}

	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

// confirmPrompt asks the user for y/N confirmation. Returns true if confirmed.
// If cfgYes is set, returns true without prompting.
func confirmPrompt(msg string) bool {
	if cfgYes {
		return true
	}
	fmt.Printf("%s (y/N): ", msg)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
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
