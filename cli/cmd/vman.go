package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	gover "github.com/hashicorp/go-version"
)

var vmanCmd = &cobra.Command{
	Use:     "vman",
	Aliases: []string{"v"},
	Short:   "Version Manager",
	Long:    `Version Manager`,
}

func verOut(cmd *cobra.Command, args []string, i int) {
	str := getInputString(cmd, args)
	act := getStringFlag(cmd, "action", false)

	v := must(gover.NewSemver(str))
	s := v.Segments()

	if act == "" {
		outputString(cmd, fmt.Sprintf("%d", s[i]))
		return
	}

	switch act {
	case "++":
		s[i] = s[i] + 1
	case "--":
		s[i] = s[i] - 1
	}

	vs := fmt.Sprintf("%d.%d.%d", s[0], s[1], s[2])
	p := v.Prerelease()
	m := v.Metadata()

	if p != "" {
		vs = fmt.Sprintf("%s-%s", vs, p)
	}

	if m != "" {
		vs = fmt.Sprintf("%s+%s", vs, m)
	}

	outputString(cmd, vs)
}

var vmajCmd = &cobra.Command{
	Use:     "major",
	Aliases: []string{"m"},
	Short:   "Major Version",
	Long:    `Major Version`,
	Run: func(cmd *cobra.Command, args []string) {
		verOut(cmd, args, 0)
	},
}

var vminCmd = &cobra.Command{
	Use:     "minor",
	Aliases: []string{"n"},
	Short:   "Minor Version",
	Long:    `Minor Version`,
	Run: func(cmd *cobra.Command, args []string) {
		verOut(cmd, args, 1)
	},
}

var vpatCmd = &cobra.Command{
	Use:     "patch",
	Aliases: []string{"p"},
	Short:   "Patch Version",
	Long:    `Patch Version`,
	Run: func(cmd *cobra.Command, args []string) {
		verOut(cmd, args, 2)
	},
}

var vpreCmd = &cobra.Command{
	Use:     "prerelease",
	Aliases: []string{"rel", "r"},
	Short:   "Prerelease",
	Long:    `Prerelease`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		act := getStringFlag(cmd, "action", false)

		v := must(gover.NewSemver(str))
		s := v.Segments()
		p := v.Prerelease()
		m := v.Metadata()

		if act == "" {
			outputString(cmd, p)
		}

		if act == "--" {
			p = ""
		} else {
			p = act
		}

		vs := fmt.Sprintf("%d.%d.%d", s[0], s[1], s[2])

		if p != "" {
			vs = fmt.Sprintf("%s-%s", vs, p)
		}

		if m != "" {
			vs = fmt.Sprintf("%s+%s", vs, m)
		}

		outputString(cmd, vs)
	},
}

var vmetCmd = &cobra.Command{
	Use:     "metadata",
	Aliases: []string{"met"},
	Short:   "Metadata",
	Long:    `Metadata`,
	Run: func(cmd *cobra.Command, args []string) {
		str := getInputString(cmd, args)
		act := getStringFlag(cmd, "action", false)

		v := must(gover.NewSemver(str))
		s := v.Segments()
		m := v.Metadata()
		p := v.Prerelease()

		if act == "" {
			outputString(cmd, m)
		}

		if act == "--" {
			m = ""
		} else {
			m = act
		}

		vs := fmt.Sprintf("%d.%d.%d", s[0], s[1], s[2])

		if p != "" {
			vs = fmt.Sprintf("%s-%s", vs, p)
		}

		if m != "" {
			vs = fmt.Sprintf("%s+%s", vs, m)
		}

		outputString(cmd, vs)
	},
}

func init() {
	rootCmd.AddCommand(vmanCmd)

	vmanCmd.AddCommand(vmajCmd)
	vmanCmd.AddCommand(vminCmd)
	vmanCmd.AddCommand(vpatCmd)
	vmanCmd.AddCommand(vpreCmd)
	vmanCmd.AddCommand(vmetCmd)

	pf := vmanCmd.PersistentFlags()
	pf.StringP("action", "a", "", "Action (++, --)")
}
