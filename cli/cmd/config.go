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
		switch typ {
		case "s", "string":
			val = str
		case "i", "int":
			val = must(strconv.ParseInt(str, 10, 64))
		case "f", "float":
			val = must(strconv.ParseFloat(str, 64))
		case "j", "json":
			exitWithError(json.Unmarshal([]byte(str), &val))
		}
		viper.Set(flag, val)
		err := viper.WriteConfig()
		if err != nil {
			fmt.Println(err)
		}
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get Configuration Value",
	Long:  `Get Configuration Value`,
	Run: func(cmd *cobra.Command, args []string) {
		flag := getInputStringOrFlag(cmd, args, "key", true)
		val := viper.Get(flag)
		byt := must(json.Marshal(val))
		outputBytes(cmd, byt)
	},
}

func init() {
	pf := configCmd.PersistentFlags()
	pf.StringP("key", "k", "", "Flag Key Path")
	pf.StringP("type", "t", "json", "Value Type (s[tring], i[nt], f[loat], j[son])")
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	rootCmd.AddCommand(configCmd)
}
