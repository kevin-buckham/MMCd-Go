package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kbuckham/mmcd/internal/sensor"
	"github.com/spf13/cobra"
)

var sensorsCmd = &cobra.Command{
	Use:   "sensors",
	Short: "List all known ECU sensors with addresses and conversions",
	Run: func(cmd *cobra.Command, args []string) {
		defs := sensor.DefaultDefinitions()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "IDX\tSLUG\tADDR\tDESCRIPTION\tUNIT\tCOMPUTED")
		fmt.Fprintln(w, "---\t----\t----\t-----------\t----\t--------")

		for i, d := range defs {
			if !d.Exists {
				continue
			}
			computed := ""
			if d.Computed {
				computed = "yes"
			}
			addrStr := fmt.Sprintf("0x%02X", d.Addr)
			if d.Addr == 0xFF {
				addrStr = "n/a"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
				i, d.Slug, addrStr, d.Description, d.Unit, computed)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(sensorsCmd)
}
