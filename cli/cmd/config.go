package cmd

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	typeJSON = "json"
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
		case "s", "string":
			val = str
		case "i", "int":
			val, err = strconv.ParseInt(str, 10, 64)
			exitWithError(cmd, err)
		case "f", "float":
			val, err = strconv.ParseFloat(str, 64)
			exitWithError(cmd, err)
		case "j", typeJSON:
			exitWithError(cmd, json.Unmarshal([]byte(str), &val))
		}
		viper.Set(flag, val)
		err = viper.WriteConfig()
		if err != nil {
			log.Println(err)
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
		byt, err := json.Marshal(val)
		exitWithError(cmd, err)
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
