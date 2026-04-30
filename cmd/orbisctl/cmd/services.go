package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage and query services in the registry",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered services",
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("%s/v1/services", getAPIURL())
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error: could not connect to registry: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Error: API returned status %d\n", resp.StatusCode)
			os.Exit(1)
		}

		var services []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
			fmt.Printf("Error: failed to parse response: %v\n", err)
			os.Exit(1)
		}

		if len(services) == 0 {
			fmt.Println("No services registered.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tADDRESS\tPORT\tHEALTH\tTAGS")

		for _, s := range services {
			id := s["id"].(string)
			name := s["name"].(string)
			addr := s["address"].(string)
			port := int(s["port"].(float64))
			health := s["health"].(string)
			
			var tags []string
			if tagsIface, ok := s["tags"].([]interface{}); ok {
				for _, t := range tagsIface {
					tags = append(tags, t.(string))
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n", id, name, addr, port, health, strings.Join(tags, ","))
		}
		w.Flush()
	},
}

var getCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get detailed information about a specific service",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		url := fmt.Sprintf("%s/v1/services/%s", getAPIURL(), id)
		resp, err := http.Get(url)
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

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error: failed to read response: %v\n", err)
			os.Exit(1)
		}

		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)
		prettyJSON, _ := json.MarshalIndent(parsed, "", "  ")
		fmt.Println(string(prettyJSON))
	},
}

func init() {
	rootCmd.AddCommand(servicesCmd)
	servicesCmd.AddCommand(listCmd)
	servicesCmd.AddCommand(getCmd)
}
