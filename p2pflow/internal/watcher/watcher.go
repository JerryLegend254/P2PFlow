package watcher

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

type ChangeEvent struct {
	SessionID string `json:"session_id"`
	User      string `json:"user"`
	Timestamp int64  `json:"ts"`
	Patch     string `json:"patch"` // diffpatch
}
type Watcher struct {
	path     string
	watcher  *fsnotify.Watcher
	last     string
	Dmp      *dmp.DiffMatchPatch
	OnChange func(patch string)
}

func NewWatcher(path string) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	d := &Watcher{path: path, watcher: w, Dmp: dmp.New()}
	// load initial
	b, _ := os.ReadFile(path)
	d.last = string(b)
	return d, nil
}

func (w *Watcher) Start() error {
	if err := w.watcher.Add(w.path); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case ev := <-w.watcher.Events:
				fmt.Println("event gotten", ev)
				if ev.Op&fsnotify.Write == fsnotify.Write {
					// small debounce
					time.Sleep(50 * time.Millisecond)
					b, _ := os.ReadFile(w.path)
					cur := string(b)
					if cur == w.last {
						continue
					}
					// create patch
					diffs := w.Dmp.DiffMain(w.last, cur, false)
					patch := w.Dmp.PatchToText(w.Dmp.PatchMake(diffs))
					w.last = cur
					if w.OnChange != nil {
						w.OnChange(patch)
					}
				}
			case err := <-w.watcher.Errors:
				log.Println("watcher error:", err)
			}
		}
	}()

	return nil
}
