package main

import (
	"github.com/JerryLegend254/p2pflow/internal/logger"
	"github.com/JerryLegend254/p2pflow/internal/watcher"
)

func main() {
	jsonLogger := logger.NewLogger(logger.JSON)
	jsonLogger.Sync()

	app := &application{
		logger: jsonLogger,
	}

	// Watch a specific test file for patch generation
	testFile := "test.txt"
	w, err := watcher.NewWatcher(testFile)
	if err != nil {
		app.logger.Fatalln(err)
	}

	// Set up OnChange callback to handle patches
	w.OnChange = func(patch string, filePath string) {
		app.logger.Infof("Patch generated for %s: %s", filePath, patch)
	}

	errCh := make(chan error)

	err = w.Start(errCh)
	if err != nil {
		app.logger.Fatalln(err)
	}

	// Handle errors from the watcher in a separate goroutine
	go func() {
		for err := range errCh {
			if err != nil {
				app.logger.Fatalln(err)
			}
		}
	}()

	// Keep the main function running
	select {}
}
