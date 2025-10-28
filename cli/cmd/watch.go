//go:build !wasm

package cmd

import (
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
)

func runCommand(cmd *cobra.Command, cmdStr string) error {
	args, err := shellwords.Parse(cmdStr)
	if err != nil {
		return err
	}

	//#nosec G204 -- implementation of generic utility
	c := exec.Command(args[0], args[1:]...)

	stdout, err := c.StdoutPipe()
	if err != nil {
		return err
	}

	err = c.Start()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		return err
	}

	err = c.Wait()
	if err != nil {
		return err
	}

	outputBytes(cmd, data)

	return nil
}

var fsWatchCmd = &cobra.Command{
	Use:     "fswatch",
	Aliases: []string{"fsw"},
	Short:   "File System Watcher",
	Long:    `File System Watcher`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdStr := getStringFlag(cmd, "cmd", true)
		dir := getStringFlag(cmd, "dir", false)
		ptn := getStringFlag(cmd, "pattern", false)

		itv := time.Duration(time.Millisecond * 250)

		limiter := rate.NewLimiter(rate.Every(itv), 1)

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			exitWithError(cmd, err)
		}
		defer func() {
			if err := watcher.Close(); err != nil {
				log.Println("Error closing watcher:", err)
			}
		}()

		done := make(chan bool)
		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if limiter.Allow() {
						if ok, err := filepath.Match(ptn, event.Name); err == nil && ok {
							err := runCommand(cmd, cmdStr)
							if err != nil {
								log.Println(err)
							}
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("error:", err)
				}
			}
		}()

		err = watcher.Add(dir)
		if err != nil {
			exitWithError(cmd, err)
		}
		<-done

	},
}

var itvWatchCmd = &cobra.Command{
	Use:     "watch",
	Aliases: []string{"itw"},
	Short:   "Interval Watcher",
	Long:    `Interval Watcher`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdStr := getStringFlag(cmd, "cmd", true)
		itv := getIntFlag(cmd, "time", false)

		ticker := time.NewTicker(time.Millisecond * time.Duration(itv))
		quit := make(chan struct{})
		go func() {
			for {
				select {
				case <-ticker.C:
					err := runCommand(cmd, cmdStr)
					if err != nil {
						log.Println(err)
					}
				case <-quit:
					ticker.Stop()
					return
				}
			}
		}()
		<-quit
	},
}

func init() {
	pf := fsWatchCmd.PersistentFlags()
	pf.StringP("dir", "d", ".", "Directory to Watch")
	pf.StringP("pattern", "p", "*", "File Pattern")
	pf.StringP("cmd", "c", "", "Command")

	pf = itvWatchCmd.PersistentFlags()
	pf.IntP("time", "t", 1000, "Interval (milliseconds)")
	pf.StringP("cmd", "c", "", "Command")

	rootCmd.AddCommand(fsWatchCmd)
	rootCmd.AddCommand(itvWatchCmd)
}
