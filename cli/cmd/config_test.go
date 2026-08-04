package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestGetProjectAndGroup(t *testing.T) {
	t.Run("flags win over config", func(t *testing.T) {
		runInTempDir(t)
		writeTestFile(t, ".sdt.yaml", "project: cfgproject\ngroup: cfggroup\n")
		c := &cobra.Command{Use: "t"}
		c.Flags().String("project", "", "")
		c.Flags().String("group", "", "")
		_ = c.Flags().Set("project", "flagproject")
		_ = c.Flags().Set("group", "flaggroup")
		p, g := getProjectAndGroup(c)
		if p != "flagproject" || g != "flaggroup" {
			t.Errorf("expected flag values, got project=%q group=%q", p, g)
		}
	})

	t.Run("config fallback", func(t *testing.T) {
		runInTempDir(t)
		writeTestFile(t, ".sdt.yaml", "project: cfgproject\ngroup: cfggroup\n")
		c := &cobra.Command{Use: "t"}
		c.Flags().String("project", "", "")
		c.Flags().String("group", "", "")
		p, g := getProjectAndGroup(c)
		if p != "cfgproject" || g != "cfggroup" {
			t.Errorf("expected config values, got project=%q group=%q", p, g)
		}
	})

	t.Run("no source", func(t *testing.T) {
		runInTempDir(t)
		c := &cobra.Command{Use: "t"}
		c.Flags().String("project", "", "")
		c.Flags().String("group", "", "")
		p, g := getProjectAndGroup(c)
		if p != "" || g != "" {
			t.Errorf("expected empty values, got project=%q group=%q", p, g)
		}
	})
}

func TestConfigSet(t *testing.T) {
	runInTempDir(t)
	execute(t, configSetCmd, []byte(`"world"`), "-k", "hello")
	val := viper.Get("hello")
	exp := `world`
	if val != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, val)
	}

	execute(t, configSetCmd, []byte("john"), "-k", "hello", "-t", "string")
	val = viper.Get("hello")
	exp = `john`
	if val != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, val)
	}

	execute(t, configSetCmd, []byte("123"), "-k", "num", "-t", "int")
	val = viper.Get("num")
	var expI int64 = 123
	if val != expI {
		t.Fatalf("expecting \"%v\", got \"%v\"", expI, val)
	}

	execute(t, configSetCmd, []byte("123.456"), "-k", "num", "-t", "float")
	val = viper.Get("num")
	expF := 123.456
	if val != expF {
		t.Fatalf("expecting \"%v\", got \"%v\"", expF, val)
	}
}

func TestConfigGet(t *testing.T) {
	viper.Set("foo", "bar")
	viper.Set("test", 123)
	out := execute(t, configGetCmd, []byte{}, "-k", "foo")
	exp := `"bar"`
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}

	out = execute(t, configGetCmd, []byte{}, "-k", "test")
	exp = `123`
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestConfigSetCommand(t *testing.T) {
	dir := runInTempDir(t)
	execute(t, configSetCmd, []byte("make all"), "-k", "tools.build", "-t", "string")
	data, err := os.ReadFile(filepath.Join(dir, ".sdt.yaml"))
	if err != nil {
		t.Fatalf(".sdt.yaml not created by config set: %v", err)
	}
	if !strings.Contains(string(data), "make all") {
		t.Errorf("expected value in .sdt.yaml:\n%s", data)
	}
}

func TestConfigShowYAML(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, ".sdt.yaml", "project: myapp\ngroup: grp\n")
	out := execute(t, configShowCmd, nil, "--format", "yaml")
	if !strings.Contains(string(out), "project: myapp") {
		t.Errorf("expected project in yaml output: %s", out)
	}
}

func TestConfigShowText(t *testing.T) {
	runInTempDir(t)
	writeTestFile(t, ".sdt.yaml", "project: myapp\ngroup: grp\n")
	out := execute(t, configShowCmd, nil)
	if !strings.Contains(string(out), "myapp") || !strings.Contains(string(out), "grp") {
		t.Errorf("expected project and group in text output: %s", out)
	}
}
