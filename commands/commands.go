package commands

import (
	"github.com/spf13/cobra"
)

var rootCmd = struct {
	*cobra.Command
	baseUrl  string
	userName string
	password string
}{
	Command: &cobra.Command{
		Use:   "md2cfl",
		Short: "Markdown to Confluence",
	},
}

func Execute() error {
	rootCmd.PersistentFlags().StringVarP(&rootCmd.baseUrl, "base", "b", "", "Confluence base URL")
	rootCmd.PersistentFlags().StringVarP(&rootCmd.userName, "user", "u", "", "Confluence user name")
	rootCmd.PersistentFlags().StringVarP(&rootCmd.password, "password", "p", "", "Confluence password")
	rootCmd.AddCommand(cmdUpload)

	return rootCmd.Execute()
}
