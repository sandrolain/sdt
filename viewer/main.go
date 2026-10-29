package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandrolain/sdt/internal/logging"
	"github.com/sandrolain/sdt/internal/semantic"
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
		Long: "Standalone browser for the SDT context knowledge base: tree, search,\n" +
			"graph, board and documents, with an optional agent chat panel.",
	}
	root.Version = fmt.Sprintf("%s (commit %s, %s)", version, commit, date)
	root.AddCommand(newServeCmd())
	return root
}

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the viewer web app over HTTP",
		Long: "Serve the viewer web app over HTTP.\n\n" +
			"Document state can be edited through the sanctioned CLI when the bind host is\n" +
			"loopback and an sdt binary resolves, and the agent chat panel is available under\n" +
			"the same loopback rule. The panel starts the agent this project declares in\n" +
			".sdt.yaml (agent.command, an argv list executed directly, with no shell) and runs it\n" +
			"on the model the same file names (agent.model, optional: absent means the agent's\n" +
			"own default, and the panel offers the agent's own list to switch it for the session):\n" +
			"nothing is started before a reader asks for it, and nothing a browser sends can\n" +
			"name a command.",
		Example: `  # the agent chat: declare the agent in the project, then open the panel (⌘J)
  printf 'agent:\n  command: ["opencode", "acp"]\n  model: "opencode/big-pickle"\n' >> .sdt.yaml
  sdtviewer serve --port 8443

  # let the viewer open the page it serves
  sdtviewer serve --port 8443 --open`,
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := cmd.Flags().GetString("host")
			if err != nil {
				return err
			}
			port, err := cmd.Flags().GetInt("port")
			if err != nil {
				return err
			}
			open, err := cmd.Flags().GetBool("open")
			if err != nil {
				return err
			}
			root, err := cmd.Flags().GetString("root")
			if err != nil {
				return err
			}
			sdtBin, err := cmd.Flags().GetString("sdt-bin")
			if err != nil {
				return err
			}
			logFormat, err := cmd.Flags().GetString("log-format")
			if err != nil {
				return err
			}
			logging.Setup(logging.Options{
				Writer: os.Stderr,
				Level:  slog.LevelInfo,
				JSON:   logFormat == "json",
				Time:   true,
			})
			var overrides semanticOverrides
			if f := cmd.Flags().Lookup("semantic"); f != nil && f.Changed {
				if v, err := cmd.Flags().GetBool("semantic"); err == nil {
					overrides.enabled = &v
				}
			}
			if f := cmd.Flags().Lookup("semantic-model"); f != nil && f.Changed {
				if v, err := cmd.Flags().GetString("semantic-model"); err == nil && v != "" {
					m := semantic.Model(v)
					overrides.model = &m
				}
			}
			return serve(host, port, open, root, sdtBin, listen, overrides)
		},
	}
	cmd.Flags().String("host", "127.0.0.1", "bind host")
	cmd.Flags().Int("port", 8443, "bind port")
	cmd.Flags().Bool("open", false, "open the platform browser at the served address")
	cmd.Flags().String("root", "", "project root; defaults to the nearest ancestor of CWD holding "+sdtConfigFile)
	cmd.Flags().String("log-format", "text", "log format: text|json")
	cmd.Flags().Bool("semantic", false, "enable the semantic search branch (overrides .sdt.yaml search.semantic); /api/search?semantic=1 then serves RRF-fused results")
	cmd.Flags().String("semantic-model", "", "embedding model for the semantic branch (default BASE8M; see `sdt models fetch` to prefetch)")
	cmd.Flags().String("sdt-bin", "", "explicit path to the sdt CLI used for document-state actions (default: PATH, then a sibling of this binary); editing is disabled when none resolves")
	return cmd
}

// serve resolves the root, builds the handler, opens the browser when `open` asks for it
// and blocks serving. listen is injectable for tests; sdtBin is the explicit
// --sdt-bin path used to resolve the sanctioned document-state mutator
// (decision 0027); overrides carries explicit --semantic/--semantic-model flag
// values that win over the project config.
func serve(host string, port int, open bool, rootFlag, sdtBin string, listen func(addr string, h http.Handler) error, overrides ...semanticOverrides) error {
	root, err := resolveRoot(rootFlag, "")
	if err != nil {
		return err
	}
	var ov *semanticOverrides
	if len(overrides) > 0 {
		ov = &overrides[0]
	}
	s, err := newServerWith(root, semanticOptionsFromConfig(root), ov)
	if err != nil {
		return err
	}
	// document editing is a delegation to the sanctioned CLI, enabled only on a
	// loopback bind host and only when a CLI resolves (decision 0027).
	s.enableWrites(host, sdtBin)
	s.enableAgent(host)
	if werr := s.startWatching(); werr != nil {
		slog.Warn("sdtviewer: live updates disabled", "err", werr)
	} else {
		defer s.stopWatching()
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	if open {
		if oerr := openBrowser("http://" + addr); oerr != nil {
			slog.Warn("sdtviewer: browser open skipped", "err", oerr)
		}
	}
	watchForShutdown(s)
	slog.Info("sdtviewer: serving", "root", root, "addr", "http://"+addr)
	return listen(addr, s.mux())
}

// watchForShutdown ends the agent before the viewer goes. The viewer started that process,
// so stopping it is the viewer's to do: a signal would otherwise end the parent and leave
// an agent that ignores a closed pipe running with nobody to talk to — the orphan the
// process group and the bounded kill exist to prevent.
//
// The signal is handled rather than swallowed: once the agent is stopped the handler is
// removed and the signal re-raised, so the process still dies of the signal it was sent
// and the shell still sees the truth.
func watchForShutdown(s *server) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signals
		slog.Info("sdtviewer: shutting down", "signal", sig)
		if err := s.agent.Close(); err != nil {
			slog.Warn("sdtviewer: closing the agent", "err", err)
		}
		// The wire log's single appended file, if the declaration asked for one,
		// closes after the bridge: the run's close mark is already in it, and the
		// file itself is the viewer's, not the run's.
		if err := s.closeAgentTrace(); err != nil {
			slog.Warn("sdtviewer: closing the agent wire log", "err", err)
		}
		signal.Stop(signals)
		if s, ok := sig.(syscall.Signal); ok {
			if self, err := os.FindProcess(os.Getpid()); err == nil {
				if err := self.Signal(s); err != nil {
					slog.Debug("re-raising the signal", "err", err)
				}
			}
		}
	}()
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
