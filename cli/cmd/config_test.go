package cmd

import (
	"testing"

	"github.com/spf13/viper"
)

func TestConfigSet(t *testing.T) {
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
