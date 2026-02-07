package cli

import (
	"fmt"

	"github.com/kbuckham/mmcd/internal/protocol"
	"github.com/kbuckham/mmcd/internal/sensor"
	"github.com/spf13/cobra"
)

var dtcErase bool

var dtcCmd = &cobra.Command{
	Use:   "dtc",
	Short: "Read and optionally erase diagnostic trouble codes (DTCs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgPort == "" {
			return fmt.Errorf("--port is required")
		}

		defs := sensor.DefaultDefinitions()
		conn := protocol.NewSerialConn(cfgPort, cfgBaud)
		if err := conn.Open(); err != nil {
			return fmt.Errorf("failed to open serial port: %w", err)
		}
		defer conn.Close()

		ecu := protocol.NewECU(conn, defs)

		result, err := ecu.ReadDTCs()
		if err != nil {
			return fmt.Errorf("failed to read DTCs: %w", err)
		}

		fmt.Println("=== Active DTCs ===")
		if len(result.Active) == 0 {
			fmt.Println("  No active faults")
		} else {
			for _, dtc := range result.Active {
				fmt.Printf("  Code %s: %s\n", dtc.Code, dtc.Description)
			}
		}
		fmt.Printf("  (raw: 0x%04X)\n", result.ActiveRaw)

		fmt.Println()
		fmt.Println("=== Stored DTCs ===")
		if len(result.Stored) == 0 {
			fmt.Println("  No stored faults")
		} else {
			for _, dtc := range result.Stored {
				fmt.Printf("  Code %s: %s\n", dtc.Code, dtc.Description)
			}
		}
		fmt.Printf("  (raw: 0x%04X)\n", result.StoredRaw)

		if dtcErase {
			fmt.Println()
			if !confirmPrompt("Erase all stored DTCs?") {
				fmt.Println("Cancelled.")
				return nil
			}
			fmt.Print("Erasing DTCs... ")
			if err := ecu.EraseDTCs(); err != nil {
				return fmt.Errorf("failed to erase DTCs: %w", err)
			}
			fmt.Println("done")
		}

		return nil
	},
}

func init() {
	dtcCmd.Flags().BoolVar(&dtcErase, "erase", false, "Erase DTCs after reading")
	rootCmd.AddCommand(dtcCmd)
}
