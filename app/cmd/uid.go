package cmd

import (
	"fmt"

	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/segmentio/ksuid"
	"github.com/spf13/cobra"
)

var uidCmd = &cobra.Command{
	Use:   "uid",
	Short: "Unique ID Tools",
	Long:  `Unique ID Tools`,
}

var uidV4Cmd = &cobra.Command{
	Use:   "v4",
	Short: "UUID v4",
	Long:  `Generate UUID v4`,
	Run: func(cmd *cobra.Command, args []string) {
		id := uuid.New()
		fmt.Print(id)
	},
}

var uidNanoCmd = &cobra.Command{
	Use:   "nano",
	Short: "Nano UID",
	Long:  `Generate Nano UID`,
	Run: func(cmd *cobra.Command, args []string) {
		id, err := gonanoid.New()
		exitWithError(err)
		fmt.Print(id)
	},
}

var uidKsCmd = &cobra.Command{
	Use:     "ks",
	Aliases: []string{"ksuid", "sortable"},
	Short:   "K-Sortable UID",
	Long:    `Generate K-Sortable UID`,
	Run: func(cmd *cobra.Command, args []string) {
		id := ksuid.New().String()
		fmt.Print(id)
	},
}

func init() {
	uidCmd.AddCommand(uidV4Cmd)
	uidCmd.AddCommand(uidNanoCmd)
	uidCmd.AddCommand(uidKsCmd)

	rootCmd.AddCommand(uidCmd)
}
