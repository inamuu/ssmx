package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"

	profile   string
	region    string
	keepalive int
	target    string
)

var rootCmd = &cobra.Command{
	Use:   "ssmx",
	Short: "AWS SSM Session Manager with configurable keepalive",
	Long:  "ssmx - AWS SSM Session Manager CLI with configurable WebSocket keepalive interval",
	RunE:  runSession,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile to use (default: AWS_PROFILE or \"default\")")
	rootCmd.PersistentFlags().StringVarP(&region, "region", "r", "", "AWS region")
	rootCmd.Flags().StringVarP(&target, "target", "t", "", "Instance ID (skip interactive selection)")
	rootCmd.Flags().IntVarP(&keepalive, "keepalive", "k", 15, "WebSocket keepalive interval in seconds")
	rootCmd.Version = version

	rootCmd.AddCommand(newRunCommand())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
