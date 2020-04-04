// Niltalk, April 2015
// License AGPL3

package main

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/niltalk/internal/hub"
	"github.com/knadh/niltalk/store/redis"
	"github.com/knadh/stuffbin"
	flag "github.com/spf13/pflag"
)

var (
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	ko     = koanf.New(".")

	// Version of the build injected at build time.
	buildString = "unknown"
)

// App is the global app context that's passed around.
type App struct {
	hub    *hub.Hub
	cfg    *hub.Config
	tpl    *template.Template
	fs     stuffbin.FileSystem
	logger *log.Logger
}

func loadConfig() {
	// Register --help handler.
	f := flag.NewFlagSet("config", flag.ContinueOnError)
	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}
	f.StringSlice("config", []string{"config.toml"},
		"Path to one or more TOML config files to load in order")
	f.Bool("new-config", false, "generate sample config file")
	f.String("static-dir", "", "(optional) path to directory with static files")
	f.Bool("version", false, "Show build version")
	f.Parse(os.Args[1:])

	// Display version.
	if ok, _ := f.GetBool("version"); ok {
		fmt.Println(buildString)
		os.Exit(0)
	}

	// Generate new config.
	if ok, _ := f.GetBool("new-config"); ok {
		if err := newConfigFile(); err != nil {
			logger.Println(err)
			os.Exit(1)
		}
		logger.Println("generated config.toml. Edit and run the app.")
		os.Exit(0)
	}

	// Read the config files.
	cFiles, _ := f.GetStringSlice("config")
	for _, f := range cFiles {
		logger.Printf("reading config: %s", f)
		if err := ko.Load(file.Provider(f), toml.Parser()); err != nil {
			if os.IsNotExist(err) {
				logger.Fatal("config file not found. If there isn't one yet, run --new-config to generate one.")
			}
			logger.Fatalf("error loadng config from file: %v.", err)
		}
	}

	// Merge env flags into config.
	if err := ko.Load(env.Provider("NILTALK_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "NILTALK_")), "__", ".", -1)
	}), nil); err != nil {
		logger.Printf("error loading env config: %v", err)
	}

	// Merge command line flags into config.
	ko.Load(posflag.Provider(f, ".", ko), nil)
}

// initFS initializes the stuffbin embedded static filesystem.
func initFS(staticDir string) stuffbin.FileSystem {
	// Get self executable path to initialise stuffed FS.
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("error getting executable path: %v", err)
	}

	// Read stuffed data from self.
	fs, err := stuffbin.UnStuff(exe)
	if err != nil {
		// Binary is unstuffed or is running in dev mode.
		// Can halt here or fall back to the local filesystem.
		if err == stuffbin.ErrNoID {
			// First argument is to the root to mount the files in the FileSystem
			// and the rest of the arguments are paths to embed.
			fs, err = stuffbin.NewLocalFS("./",
				"./static/templates",
				"./static/static:/static",
				"config.toml.sample")
			if err != nil {
				log.Fatalf("error falling back to local filesystem: %v", err)
			}
		} else {
			log.Fatalf("error reading stuffed binary: %v", err)
		}
	}

	// Optional static directory to override files.
	if staticDir != "" {
		logger.Printf("loading static files from: %v", staticDir)
		fStatic, err := stuffbin.NewLocalFS("/",
			filepath.Join(staticDir, "/templates")+":/static/templates",
			filepath.Join(staticDir, "/static")+":/static",
		)
		if err != nil {
			logger.Fatalf("failed reading static directory: %s: %v", staticDir, err)
		}
		if err := fs.Merge(fStatic); err != nil {
			logger.Fatalf("error merging static directory: %s: %v", staticDir, err)
		}
	}
	return fs
}

// Catch OS interrupts and respond accordingly.
// This is not fool proof as http keeps listening while
// existing rooms are shut down.
func catchInterrupts() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		for sig := range c {
			// Shutdown.
			logger.Printf("shutting down: %v", sig)
			os.Exit(0)
		}
	}()
}

func newConfigFile() error {
	if _, err := os.Stat("config.toml"); !os.IsNotExist(err) {
		return errors.New("config.toml exists. Remove it to generate a new one")
	}

	// Initialize the static file system into which all
	// required static assets (.sql, .js files etc.) are loaded.
	fs := initFS("")
	b, err := fs.Read("config.toml.sample")
	if err != nil {
		return fmt.Errorf("error reading sample config (is binary stuffed?): %v", err)
	}

	return ioutil.WriteFile("config.toml", b, 0644)
}

func main() {
	// Load configuration from files.
	loadConfig()

	// Initialize global app context.
	app := &App{
		logger: logger,
		fs:     initFS(ko.String("static-dir")),
	}
	if err := ko.Unmarshal("app", &app.cfg); err != nil {
		logger.Fatalf("error unmarshalling 'app' config: %v", err)
	}

	minTime := time.Duration(3) * time.Second
	if app.cfg.RoomAge < minTime || app.cfg.WSTimeout < minTime {
		logger.Fatal("app.websocket_timeout and app.roomage should be > 3s")
	}

	// Initialize store.
	var storeCfg redis.Config
	if err := ko.Unmarshal("store", &storeCfg); err != nil {
		logger.Fatalf("error unmarshalling 'store' config: %v", err)
	}

	store, err := redis.New(storeCfg)
	if err != nil {
		log.Fatalf("error initializing store: %v", err)
	}
	app.hub = hub.NewHub(app.cfg, store, logger)

	// Compile static templates.
	tpl, err := stuffbin.ParseTemplatesGlob(nil, app.fs, "/static/templates/*.html")
	if err != nil {
		logger.Fatalf("error compiling templates: %v", err)
	}
	app.tpl = tpl

	// Register HTTP routes.
	r := chi.NewRouter()
	r.Get("/", wrap(handleIndex, app, 0))
	r.Get("/ws/{roomID}", wrap(handleWS, app, hasAuth|hasRoom))

	// API.
	r.Post("/api/rooms/{roomID}/login", wrap(handleLogin, app, hasRoom))
	r.Delete("/api/rooms/{roomID}/login", wrap(handleLogout, app, hasAuth|hasRoom))
	r.Post("/api/rooms", wrap(handleCreateRoom, app, 0))

	// Views.
	r.Get("/r/{roomID}", wrap(handleRoomPage, app, hasAuth|hasRoom))
	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		app.fs.FileServer().ServeHTTP(w, r)
	})

	// Start the app.
	srv := &http.Server{
		Addr:    ko.String("app.address"),
		Handler: r,
	}
	logger.Printf("starting server on %v", ko.String("app.address"))
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("couldn't start server: %v", err)
	}
}
