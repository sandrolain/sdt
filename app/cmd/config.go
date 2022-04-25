package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg"},
	Short:   "Configuration Tools",
	Long:    `Configuration Tools`,
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set Configuration Value",
	Long:  `Set Configuration Value`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		flag := getStringFlag(cmd, "key", true)
		typ := getStringFlag(cmd, "type", false)
		var val any
		var err error
		switch typ {
		default:
			val = str
		case "int":
		case "i":
			val, err = strconv.ParseInt(str, 10, 64)
		case "float":
		case "f":
			val, err = strconv.ParseFloat(str, 64)
		case "json":
		case "j":
			err = json.Unmarshal([]byte(str), &val)
		}
		exitWithError(err)

		viper.Set(flag, val)
		viper.WriteConfig()
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get Configuration Value",
	Long:  `Get Configuration Value`,
	Run: func(cmd *cobra.Command, args []string) {
		flag := getInputStringOrFlag(cmd, args, "key", true)
		val := viper.Get(flag)
		byt, err := json.Marshal(val)
		exitWithError(err)
		fmt.Print(string(byt))
	},
}

func init() {
	pf := configCmd.PersistentFlags()
	pf.StringP("key", "k", "", "Flag Key Path")
	pf.StringP("type", "t", "string", "Value Type (string, int, float, json)")
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	rootCmd.AddCommand(configCmd)
}
