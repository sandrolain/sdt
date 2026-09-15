package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// sdtConfigFile is the project marker file used for discovery.
const sdtConfigFile = ".sdt.yaml"

var (
	// listen is the HTTP serve entry (timeout-guarded per gosec G114);
	// injectable for tests.
	listen = func(addr string, h http.Handler) error {
		srv := &http.Server{
			Addr:              addr,
			Handler:           h,
			ReadHeaderTimeout: 30 * time.Second,
		}
		return srv.ListenAndServe()
	}
	// runCmd starts a process; injectable for tests.
	runCmd = func(name string, args ...string) error {
		//#nosec G204 -- fixed browser launcher, URL is our own localhost
		return exec.Command(name, args...).Start()
	}
	// openBrowser opens the platform browser; injectable for tests.
	openBrowser = func(url string) error {
		name, args := browserCmd(runtime.GOOS, url)
		return runCmd(name, args...)
	}
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run executes the root command; factored out of main for testability.
func run() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "sdtviewer",
		Short: "SDT context/ wiki browser viewer",
		Long:  "Standalone read-only browser for the SDT context knowledge base.",
	}
	root.Version = fmt.Sprintf("%s (commit %s, %s)", version, commit, date)
	root.AddCommand(newServeCmd())
	return root
}

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the viewer web app over HTTP",
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := cmd.Flags().GetString("host")
			if err != nil {
				return err
			}
			port, err := cmd.Flags().GetInt("port")
			if err != nil {
				return err
			}
			noOpen, err := cmd.Flags().GetBool("no-open")
			if err != nil {
				return err
			}
			root, err := cmd.Flags().GetString("root")
			if err != nil {
				return err
			}
			logFormat, err := cmd.Flags().GetString("log-format")
			if err != nil {
				return err
			}
			setupLogger(logFormat)
			return serve(host, port, noOpen, root, listen)
		},
	}
	cmd.Flags().String("host", "127.0.0.1", "bind host")
	cmd.Flags().Int("port", 8443, "bind port")
	cmd.Flags().Bool("no-open", false, "do not open the browser")
	cmd.Flags().String("root", "", "project root; defaults to the nearest ancestor of CWD holding "+sdtConfigFile)
	cmd.Flags().String("log-format", "text", "log format: text|json")
	return cmd
}

// setupLogger installs the slog default logger in text or json format.
func setupLogger(format string) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// serve resolves the root, builds the handler, optionally opens the browser and
// blocks serving. listen is injectable for tests.
func serve(host string, port int, noOpen bool, rootFlag string, listen func(addr string, h http.Handler) error) error {
	root, err := resolveRoot(rootFlag, "")
	if err != nil {
		return err
	}
	s, err := newServer(root)
	if err != nil {
		return err
	}
	if werr := s.startWatching(); werr != nil {
		slog.Warn("sdtviewer: live updates disabled", "err", werr)
	} else {
		defer s.stopWatching()
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	if !noOpen {
		if oerr := openBrowser("http://" + addr); oerr != nil {
			slog.Warn("sdtviewer: browser open skipped", "err", oerr)
		}
	}
	slog.Info("sdtviewer: serving", "root", root, "addr", "http://"+addr)
	return listen(addr, s.mux())
}

// resolveRoot returns the directory to serve: an explicit --root, or the
// nearest ancestor of cwd holding .sdt.yaml, walking up like .git. cwd may be
// passed explicitly for tests; "" resolves to os.Getwd().
func resolveRoot(rootFlag, cwd string) (string, error) {
	if rootFlag != "" {
		abs, err := filepath.Abs(rootFlag)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("resolve --root: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("resolve --root: %s is not a directory", abs)
		}
		return abs, nil
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, sdtConfigFile)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no %s found upward from %s and no --root given; use --root <dir> or run inside a project", sdtConfigFile, cwd)
}

// browserCmd returns the launcher command for the given OS and URL.
func browserCmd(goos, url string) (string, []string) {
	switch goos {
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		return "open", []string{url}
	default:
		return "xdg-open", []string{url}
	}
}
