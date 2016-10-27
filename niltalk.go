// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/alexedwards/stack"
	"github.com/bmizerany/pat"
	"github.com/gorilla/websocket"
	"github.com/knadh/jsonconfig"
)

var (
	dbPool   *DBpool
	config   *Configuration
	upgrader websocket.Upgrader
	Logger   *log.Logger
)

func init() {
	// Load configuration from file.
	configFile := flag.String("config", "config.json", "Configuration file")

	// Command line arguments.
	flag.Parse()

	err := jsonconfig.Load(*configFile, &config)
	if err != nil {
		panic(fmt.Sprintf("Error parsing or reading the config: %v", err))
	}

	// Initialisations.
	dbPool = NewDBpool(config.CacheAddress, config.CachePassword, config.CachePoolActive, config.CachePoolIdle)

	upgrader = websocket.Upgrader{
		ReadBufferSize:  config.ReadBuffer,
		WriteBufferSize: config.WriteBuffer,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	initLogger(config.Debug)
}

// Custom file/screen message logger.
func initLogger(debug bool) {
	var file_handle io.Writer

	if debug {
		file_handle = os.Stdout
	} else {
		file_handle = ioutil.Discard
	}

	Logger = log.New(file_handle, "DEBUG: ", log.Ldate|log.Ltime)
}

// Periodic release of unused memory back to the OS.
// Long running network daemons, especially that see numerous TCP connections
// open and close end up consuming a lot of memory which doesn't seem to be
// freed in an efficient manner.
func periodMemoryRelease(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	go func() {
		for _ = range ticker.C {
			Logger.Println("FreeOSMemory()")
			debug.FreeOSMemory()
		}
	}()
}

// Catch OS interrupts and respond accordingly.
// This is not fool proof as http keeps listening while
// existing rooms are shut down.
func catchInterrupts() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGUSR1)
	go func() {
		for sig := range c {
			if sig == syscall.SIGUSR1 {
				// Reload assets.
				Logger.Println("Reloading assets")
				loadAssets()
			} else {
				// Shutdown.
				Logger.Println("Shutting down:", sig)
				shutdown()
				os.Exit(0)
			}
		}
	}()
}

// Shut the app down by terminating all rooms.
// The shutdown is triggered by an OS signal.
func shutdown() {
	for r := range rooms {
		rooms[r].stop <- websocket.CloseInternalServerErr
	}
}

// Setup http routes.
func setupRoutes(config *Configuration) {
	r := pat.New()

	// Websocket requests.
	r.Get(config.WebsocketRoute+":room_id", stack.New(hasRoom, hasAuth).Then(webSocketHandler))

	// Homepage.
	r.Get("/", http.HandlerFunc(indexPage))

	// Other pages.
	r.Get("/pages/:page_id", http.HandlerFunc(staticPage))

	// Room related routes.
	r.Post(config.RoomRoute+"create", http.HandlerFunc(createRoom))
	r.Get(config.RoomRoute+":room_id", stack.New(hasRoom).Then(roomPage))
	r.Post(config.RoomRoute+":room_id/login", stack.New(hasRoom).Then(login))
	r.Post(config.RoomRoute+":room_id/dispose", stack.New(hasRoom, hasAuth).Then(disposeRoom))

	// Debug stats.
	r.Get("/debug", http.HandlerFunc(debugPage))

	http.Handle("/", r)
}

func main() {
	// Startup db connectivity test.
	db := dbPool.Get()
	if db.conn.Err() != nil {
		panic("Redis connection failed")
	}
	db.Close()
	defer dbPool.Close()

	// Start the memory release service.
	periodMemoryRelease(config.MemoryReleaseInterval)
	catchInterrupts()

	// Load assets.
	loadAssets()

	// Setup http routes.
	setupRoutes(config)

	fmt.Println("Starting server on", config.Address)

	// Static files.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start the server.
	if err := http.ListenAndServe(config.Address, nil); err != nil {
		Logger.Fatalln("ListenAndServe:", err)
	}
}

// Some debug information over http.
func debugPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Total peers.
	np := 0
	for r := range rooms {
		np += rooms[r].peerCount()
	}

	content := `
Active rooms = %d
Connected peers = %d
Goroutines = %d
`

	io.WriteString(w, fmt.Sprintf(
		content,
		len(rooms),
		np,
		runtime.NumGoroutine()))
}
