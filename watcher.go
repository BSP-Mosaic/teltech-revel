package revel

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rjeczalik/notify"
	"github.com/BSP-Mosaic/teltech-glog"
)

// Listener is an interface for receivers of filesystem events.
type Listener interface {
	// Refresh is invoked by the watcher on relevant filesystem events.
	// If the listener returns an error, it is served to the user on the current request.
	Refresh() *Error
}

// DiscerningListener allows the receiver to selectively watch files.
type DiscerningListener interface {
	Listener
	WatchDir(basename string) bool
	WatchFile(basename string) bool
}

// Watcher allows listeners to register to be notified of changes under a given
// directory.
type Watcher struct {
	// Parallel arrays of watcher/listener pairs.
	events       []chan notify.EventInfo
	listeners    []Listener
	forceRefresh bool
	lastError    int
	notifyMutex  sync.Mutex
}

func NewWatcher() *Watcher {
	return &Watcher{
		forceRefresh: true,
		lastError:    -1,
	}
}

// Listen registers for events within the given root directories (recursively).
func (w *Watcher) Listen(listener Listener, roots ...string) {
	eventCh := make(chan notify.EventInfo, 100)

	// Walk through all files / directories under the root, adding each to watcher.
	for _, p := range roots {
		// is the directory / file a symlink?
		f, err := os.Lstat(p)
		if err == nil && f.Mode()&os.ModeSymlink == os.ModeSymlink {
			realPath, err := filepath.EvalSymlinks(p)
			if err != nil {
				panic(err)
			}
			p = realPath
		}

		fi, err := os.Stat(p)
		if err != nil {
			glog.Errorln("Failed to stat watched path", p, ":", err)
			continue
		}

		if fi.IsDir() {
			err = notify.Watch(p+string(filepath.Separator)+"...", eventCh, notify.All)
		} else {
			err = notify.Watch(p, eventCh, notify.All)
		}
		if err != nil {
			glog.Errorln("Failed to watch", p, ":", err)
		}
	}

	if w.eagerRebuildEnabled() {
		// Create goroutine to notify file changes in real time
		go w.NotifyWhenUpdated(listener, eventCh)
	}

	w.events = append(w.events, eventCh)
	w.listeners = append(w.listeners, listener)
}

// NotifyWhenUpdated notifies the watcher when a file event is received.
func (w *Watcher) NotifyWhenUpdated(listener Listener, eventCh chan notify.EventInfo) {
	for {
		select {
		case ev := <-eventCh:
			if w.rebuildRequired(ev, listener) {
				// Serialize listener.Refresh() calls.
				w.notifyMutex.Lock()
				listener.Refresh()
				w.notifyMutex.Unlock()
			}
		}
	}
}

// Notify causes the watcher to forward any change events to listeners.
// It returns the first (if any) error returned.
func (w *Watcher) Notify() *Error {
	// Serialize Notify() calls.
	w.notifyMutex.Lock()
	defer w.notifyMutex.Unlock()

	for i, eventCh := range w.events {
		listener := w.listeners[i]

		// Pull all pending events / errors from the watcher.
		refresh := false
		for {
			select {
			case ev := <-eventCh:
				if w.rebuildRequired(ev, listener) {
					refresh = true
				}
				continue
			default:
				// No events left to pull
			}
			break
		}

		if w.forceRefresh || refresh || w.lastError == i {
			err := listener.Refresh()
			if err != nil {
				w.lastError = i
				return err
			}
		}
	}

	w.forceRefresh = false
	w.lastError = -1
	return nil
}

// If watcher.mode is set to eager, the application is rebuilt immediately
// when a source file is changed.
// This feature is available only in dev mode.
func (w *Watcher) eagerRebuildEnabled() bool {
	return Config.BoolDefault("mode.dev", true) &&
		Config.BoolDefault("watch", true) &&
		Config.StringDefault("watcher.mode", "normal") == "eager"
}

func (w *Watcher) rebuildRequired(ev notify.EventInfo, listener Listener) bool {
	// Ignore changes to dotfiles.
	if strings.HasPrefix(path.Base(ev.Path()), ".") {
		return false
	}

	if dl, ok := listener.(DiscerningListener); ok {
		if !dl.WatchDir(path.Base(path.Dir(ev.Path()))) || !dl.WatchFile(path.Base(ev.Path())) {
			return false
		}
	}
	return true
}

var WatchFilter = func(c *Controller, fc []Filter) {
	if MainWatcher != nil {
		err := MainWatcher.Notify()
		if err != nil {
			c.Result = c.RenderError(err)
			return
		}
	}
	fc[0](c, fc[1:])
}
