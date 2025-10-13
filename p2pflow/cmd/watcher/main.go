package main

import (
	"github.com/JerryLegend254/p2pflow/internal/logger"
	"github.com/fsnotify/fsnotify"
)

func main() {
	jsonLogger := logger.NewLogger(logger.JSON)
	jsonLogger.Sync()

	app := &application{
		logger: jsonLogger,
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		app.logger.Fatalln(err)
	}
	defer watcher.Close()

	err = watcher.Add(".")
	if err != nil {
		app.logger.Fatalln(err)
	}

	errCh := make(chan error)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				switch {
				case event.Op&fsnotify.Write == fsnotify.Write:
					app.logger.Infof("Write:  %s: %s", event.Op, event.Name)
				case event.Op&fsnotify.Create == fsnotify.Create:
					app.logger.Infof("Create: %s: %s", event.Op, event.Name)
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					app.logger.Infof("Remove: %s: %s", event.Op, event.Name)
				case event.Op&fsnotify.Rename == fsnotify.Rename:
					app.logger.Infof("Rename: %s: %s", event.Op, event.Name)
				case event.Op&fsnotify.Chmod == fsnotify.Chmod:
					app.logger.Infof("Chmod:  %s: %s", event.Op, event.Name)
				}
			case err := <-watcher.Errors:
				errCh <- err
			}
		}
	}()

	app.logger.Fatalln(<-errCh)
}
