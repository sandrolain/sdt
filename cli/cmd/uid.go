package cmd

import (
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/segmentio/ksuid"
	"github.com/spf13/cobra"
	"uuid"
)

var uidCmd = &cobra.Command{
	Use:   "uid",
	Short: "Unique ID",
	Long:  `Unique Identifier`,
}

var uidV4Cmd = &cobra.Command{
	Use:   "v4",
	Short: "UUID v4",
	Long:  `Generate UUID v4 (stdlib uuid; random)`,
	Run: func(cmd *cobra.Command, args []string) {
		outputString(cmd, uuid.NewV4().String())
	},
}

var uidV7Cmd = &cobra.Command{
	Use:   "v7",
	Short: "UUID v7",
	Long:  `Generate UUID v7 (stdlib uuid; RFC 9562, time-ordered)`,
	Run: func(cmd *cobra.Command, args []string) {
		outputString(cmd, uuid.NewV7().String())
	},
}

var uidNanoCmd = &cobra.Command{
	Use:   "nano",
	Short: "Nano UID",
	Long:  `Generate Nano UID`,
	Run: func(cmd *cobra.Command, args []string) {
		id, err := gonanoid.New()
		exitWithError(cmd, err)
		outputString(cmd, id)
	},
}

var uidKsCmd = &cobra.Command{
	Use:     "ks",
	Aliases: []string{"ksuid", "sortable"},
	Short:   "K-Sortable UID",
	Long:    `Generate K-Sortable UID`,
	Run: func(cmd *cobra.Command, args []string) {
		id := ksuid.New().String()
		outputString(cmd, id)
	},
}

func init() {
	uidCmd.AddCommand(uidV4Cmd)
	uidCmd.AddCommand(uidV7Cmd)
	uidCmd.AddCommand(uidNanoCmd)
	uidCmd.AddCommand(uidKsCmd)
	rootCmd.AddCommand(uidCmd)
}
