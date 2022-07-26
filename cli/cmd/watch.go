//go:build !wasm

package cmd

import (
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
)

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
			log.Fatal(err)
		}
		defer watcher.Close()

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
							args, err := shellwords.Parse(cmdStr)
							if err != nil {
								log.Println(err)
								continue
							}

							c := exec.Command(args[0], args[1:]...)

							stdout, err := c.StdoutPipe()
							if err != nil {
								log.Println(err)
								continue
							}

							err = c.Start()
							if err != nil {
								log.Println(err)
								continue
							}

							data, err := ioutil.ReadAll(stdout)
							if err != nil {
								log.Println(err)
								continue
							}

							err = c.Wait()
							if err != nil {
								log.Println(err)
								continue
							}

							outputBytes(cmd, data)
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
			log.Fatal(err)
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
		// TODO
	},
}

func init() {
	fsWatchCmd.PersistentFlags().StringP("dir", "d", ".", "Directory to Watch")
	fsWatchCmd.PersistentFlags().StringP("pattern", "p", "*", "File Pattern")
	fsWatchCmd.PersistentFlags().StringP("cmd", "c", "", "Command")
	rootCmd.AddCommand(fsWatchCmd)
	rootCmd.AddCommand(itvWatchCmd)
}
