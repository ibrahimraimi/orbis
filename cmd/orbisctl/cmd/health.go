package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Manage health checks",
}

var triggerCmd = &cobra.Command{
	Use:   "trigger [id]",
	Short: "Trigger a manual heartbeat for a service",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		url := fmt.Sprintf("%s/v1/services/%s/heartbeat", getAPIURL(), id)
		
		req, err := http.NewRequest(http.MethodPut, url, nil)
		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			os.Exit(1)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error: could not connect to registry: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			fmt.Printf("Service '%s' not found.\n", id)
			os.Exit(1)
		}
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Error: API returned status %d\n", resp.StatusCode)
			os.Exit(1)
		}

		fmt.Printf("Successfully sent heartbeat for service '%s'.\n", id)
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
	healthCmd.AddCommand(triggerCmd)
}
