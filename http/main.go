package main

import (
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"
	"github.com/sandrolain/sdt/cli/cmd"
)

func endWithError(w http.ResponseWriter, err error) {
	w.WriteHeader(400)
	_, e := w.Write([]byte(err.Error()))
	if e != nil {
		log.Fatal(err)
	}
}

func main() {
	http.HandleFunc("/sdt/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[5:]
		path = strings.Trim(path, "/")

		args, err := shellwords.Parse(path)
		if err != nil {
			endWithError(w, err)
			return
		}

		query := r.URL.Query()

		var in []byte
		if r.Method == http.MethodPost {
			in, err = io.ReadAll(r.Body)
			if err != nil {
				endWithError(w, err)
				return
			}
		} else {
			in = []byte(r.URL.Query().Get("in"))
		}

		for k, v := range query {
			if k == "in" {
				continue
			}
			if len(k) == 1 {
				k = "-" + k
			} else {
				k = "--" + k
			}
			args = append(args, k)
			args = append(args, strings.Join(v, ""))
		}

		out, err := cmd.ExecuteByArgs(args, in)
		if err != nil {
			endWithError(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		_, err = w.Write(out)
		if err != nil {
			log.Fatal(err)
		}
	})

	server := &http.Server{
		Addr:              ":8090",
		ReadHeaderTimeout: 3 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}
