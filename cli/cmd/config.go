package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/goccy/go-yaml"
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

// configInitCmd creates a .sdt.yaml with project identity.
var configInitCmd = &cobra.Command{
	Use:   useInit,
	Short: "Initialize .sdt.yaml with project identity",
	Long: `Create a .sdt.yaml file in the current directory.

The file stores the project identity used by project-scoped commands:
  project     — project name
  group       — group name

Examples:
  sdt config init --project myapp
  sdt config init --project myapp --group platform`,
	Run: func(cmd *cobra.Command, args []string) {
		project := getStringFlag(cmd, "project", false)
		if project == "" {
			exitWithError(cmd, fmt.Errorf("--project is required"))
			return
		}
		group := getStringFlag(cmd, "group", false)
		force := getBoolFlag(cmd, "force", false)

		if _, err := os.Stat(sdtConfigFile); err == nil && !force {
			exitWithError(cmd, fmt.Errorf("%s already exists (use --force to overwrite)", sdtConfigFile))
			return
		}

		content := buildProjectConfigContent(project, group)
		if err := os.WriteFile(sdtConfigFile, []byte(content), 0o600); err != nil { //#nosec G306 -- user project config
			exitWithError(cmd, err)
		}
		outputString(cmd, fmt.Sprintf("created %s", sdtConfigFile))
	},
}

// configShowCmd prints the resolved project configuration.
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show project configuration",
	Long: `Print the project configuration resolved from .sdt.yaml found by walking
up from the current directory (like .git).`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := findProjectConfig()
		exitWithError(cmd, err)
		if cfg == nil {
			exitWithError(cmd, fmt.Errorf("no %s found walking up from the current directory", sdtConfigFile))
			return
		}
		switch getFormat(cmd) {
		case fmtJSON:
			out, err := json.MarshalIndent(cfg, "", "  ")
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		case fmtYAML:
			out, err := yaml.Marshal(cfg)
			exitWithError(cmd, err)
			outputBytes(cmd, out)
		default:
			outputString(cmd, fmt.Sprintf("project:  %s\ngroup:    %s\n", cfg.Project, cfg.Group))
		}
	},
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
		case "s", typeString:
			val = str
		case "i", typeInt:
			val, err = strconv.ParseInt(str, 10, 64)
			exitWithError(cmd, err)
		case "f", "float":
			val, err = strconv.ParseFloat(str, 64)
			exitWithError(cmd, err)
		case "j", typeJSON:
			exitWithError(cmd, json.Unmarshal([]byte(str), &val))
		}
		viper.Set(flag, val)
		if _, statErr := os.Stat(sdtConfigFile); statErr == nil {
			err = viper.WriteConfig()
		} else {
			err = viper.WriteConfigAs(sdtConfigFile)
		}
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
	configInitCmd.Flags().String("project", "", "Project name")
	configInitCmd.Flags().String("group", "", "Group name")
	configInitCmd.Flags().Bool("force", false, "Overwrite existing .sdt.yaml")

	configCmd.PersistentFlags().StringP("key", "k", "", "Flag Key Path")
	configCmd.PersistentFlags().StringP("type", "t", "json", "Value Type (s[tring], i[nt], f[loat], j[son])")
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	rootCmd.AddCommand(configCmd)
}
