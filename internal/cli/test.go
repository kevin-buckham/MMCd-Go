package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/kbuckham/mmcd/internal/protocol"
	"github.com/kbuckham/mmcd/internal/sensor"
	"github.com/spf13/cobra"
)

// Actuator command mapping
var actuatorCommands = map[string]struct {
	addr byte
	desc string
}{
	"fuel-pump": {0xF6, "Fuel pump relay"},
	"purge":     {0xF5, "Canister purge solenoid"},
	"pressure":  {0xF4, "Pressure solenoid"},
	"egr":       {0xF3, "EGR solenoid"},
	"mvic":      {0xF2, "MVIC motor"},
	"boost":     {0xF1, "Boost solenoid"},
	"inj1":      {0xFC, "Disable injector #1"},
	"inj2":      {0xFB, "Disable injector #2"},
	"inj3":      {0xFA, "Disable injector #3"},
	"inj4":      {0xF9, "Disable injector #4"},
	"inj5":      {0xF8, "Disable injector #5"},
	"inj6":      {0xF7, "Disable injector #6"},
}

var testCommand string

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run actuator tests (fuel pump, purge, injector disable, etc.)",
	Long: `Sends actuator test commands to the ECU.
Solenoid/relay commands (fuel-pump, purge, etc.) only work with engine OFF.
Injector disable commands work with engine running.

The ECU activates the component for ~6 seconds then responds.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgPort == "" {
			return fmt.Errorf("--port is required")
		}

		if testCommand == "" {
			// List available commands
			fmt.Println("Available actuator test commands:")
			fmt.Println()
			fmt.Println("  Solenoids/Relays (engine OFF only):")
			for _, name := range []string{"fuel-pump", "purge", "pressure", "egr", "mvic", "boost"} {
				c := actuatorCommands[name]
				fmt.Printf("    %-12s  0x%02X  %s\n", name, c.addr, c.desc)
			}
			fmt.Println()
			fmt.Println("  Injector Disable (engine running):")
			for _, name := range []string{"inj1", "inj2", "inj3", "inj4", "inj5", "inj6"} {
				c := actuatorCommands[name]
				fmt.Printf("    %-12s  0x%02X  %s\n", name, c.addr, c.desc)
			}
			fmt.Println()
			fmt.Println("Usage: mmcd test --command <name>")
			return nil
		}

		ac, ok := actuatorCommands[strings.ToLower(testCommand)]
		if !ok {
			return fmt.Errorf("unknown test command: %s", testCommand)
		}

		defs := sensor.DefaultDefinitions()
		conn := protocol.NewSerialConn(cfgPort, cfgBaud)
		if err := conn.Open(); err != nil {
			return fmt.Errorf("failed to open serial port: %w", err)
		}
		defer conn.Close()

		ecu := protocol.NewECU(conn, defs)

		fmt.Printf("Sending: %s (0x%02X) — %s\n", testCommand, ac.addr, ac.desc)
		fmt.Println("Waiting for ECU response (~6 seconds)...")

		result, err := ecu.SendCommand(ac.addr, 7*time.Second)
		if err != nil {
			return fmt.Errorf("test command failed: %w", err)
		}

		fmt.Printf("ECU response: 0x%02X", result)
		if result == 0x00 {
			fmt.Println(" (OK)")
		} else if result == 0xFF {
			fmt.Println(" (engine running — solenoid commands require engine OFF)")
		} else {
			fmt.Println()
		}

		return nil
	},
}

func init() {
	testCmd.Flags().StringVarP(&testCommand, "command", "c", "", "Actuator test command name")
	rootCmd.AddCommand(testCmd)
}
