package revel

import (
	"fmt"
	"net/http"
	"time"

	"github.com/BSP-Mosaic/teltech-glog"
	"golang.org/x/net/websocket"
)

var (
	MainRouter         *Router
	MainTemplateLoader *TemplateLoader
	MainWatcher        *Watcher
	Server             *http.Server
)

// This method handles all requests.  It dispatches to handleInternal after
// handling / adapting websocket connections.
func handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		websocket.Handler(func(ws *websocket.Conn) {
			r.Method = "WS"
			handleInternal(w, r, ws)
		}).ServeHTTP(w, r)
	} else {
		handleInternal(w, r, nil)
	}
}

func handleInternal(w http.ResponseWriter, r *http.Request, ws *websocket.Conn) {
	var (
		req  = NewRequest(r)
		resp = NewResponse(w)
		c    = NewController(req, resp, ws)
	)

	Filters[0](c, Filters[1:])
	if c.Result != nil {
		c.Result.Apply(req, resp)
	} else if c.Response.Status != 0 {
		c.Response.Out.WriteHeader(c.Response.Status)
	}
}

// Run the server.
// This is called from the generated main file.
// If port is non-zero, use that.  Else, read the port from app.conf.
func Run(port int) {
	address := HttpAddr
	if port == 0 {
		port = HttpPort
	}

	MainTemplateLoader = NewTemplateLoader(TemplatePaths)

	// The "watch" config variable can turn on and off all watching.
	// (As a convenient way to control it all together.)
	if Config.BoolDefault("watch", true) {
		MainWatcher = NewWatcher()
		Filters = append([]Filter{WatchFilter}, Filters...)
	}

	// If desired (or by default), create a watcher for templates and routes.
	// The watcher calls Refresh() on things on the first request.
	if MainWatcher != nil && Config.BoolDefault("watch.templates", true) {
		MainWatcher.Listen(MainTemplateLoader, TemplatePaths...)
	} else {
		MainTemplateLoader.Refresh()
	}

	Server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", address, port),
		Handler: http.HandlerFunc(handle),
	}

	runStartupHooks()

	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("Listening on port %d...\n", port)
	}()

	if HttpSsl {
		glog.Fatalln("Failed to listen:",
			Server.ListenAndServeTLS(HttpSslCert, HttpSslKey))
	} else {
		glog.Fatalln("Failed to listen:", Server.ListenAndServe())
	}
}

func runStartupHooks() {
	for _, hook := range startupHooks {
		hook()
	}
}

var startupHooks []func()

func OnAppStart(f func()) {
	startupHooks = append(startupHooks, f)
}
