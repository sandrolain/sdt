package cmd

import (
	"fmt"
	"testing"
	"time"
)

func TestTimeUnix(t *testing.T) {
	out := execute(t, timeUnixCmd, []byte{})
	exp := fmt.Sprint(time.Now().Unix())
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}

	in := "1656246748"
	out = execute(t, timeUnixCmd, []byte{}, "-t", in)
	exp = "1656246748"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}

	in = "1656246748123"
	out = execute(t, timeUnixCmd, []byte{}, "-m", "-t", in)
	exp = "1656246748"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestTimeISO(t *testing.T) {
	in := "1656246748123"
	out := execute(t, timeIsoCmd, []byte{}, "-m", "-t", in)
	exp := "2022-06-26T12:32:28Z"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestTimeHTTP(t *testing.T) {
	in := "1656246748123"
	out := execute(t, timeHttpCmd, []byte{}, "-m", "-t", in)
	exp := "Sun, 26 Jun 2022 12:32:28 GMT"
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}
