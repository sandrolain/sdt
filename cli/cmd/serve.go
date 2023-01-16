package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/spf13/cobra"
)

func customNotFound(fs http.FileSystem, file string) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fs.Open(path.Clean(r.URL.Path)) // Do not allow path traversals.
		if os.IsNotExist(err) {
			if file != "" {
				index, err := fs.Open(file)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, "%s not found", file)
					return
				}

				fi, err := index.Stat()
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, "%s not found", file)
					return
				}

				http.ServeContent(w, r, fi.Name(), fi.ModTime(), index)
				return
			}
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Static File Server",
	Long:  `Static File Server`,
	Run: func(cmd *cobra.Command, args []string) {
		path := getStringFlag(cmd, "dir", false)
		port := getIntFlag(cmd, "port", false)
		spa := getStringFlag(cmd, "spa", false)
		outputString(cmd, fmt.Sprintf("Listening on :%v...", port))

		server := &http.Server{
			Addr:              fmt.Sprintf(":%v", port),
			ReadHeaderTimeout: 3 * time.Second,
		}

		server.Handler = customNotFound(http.Dir(path), spa)

		exitWithError(server.ListenAndServe())
	},
}

func init() {
	pf := serveCmd.PersistentFlags()
	pf.StringP("dir", "d", ".", "Directory Path")
	pf.IntP("port", "p", 3000, "Port")
	pf.StringP("spa", "s", "", "Default file to serve for SPA")
	rootCmd.AddCommand(serveCmd)
}
