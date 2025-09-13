package main // import "github.com/mvanholsteijn/paas-monitor"

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/shirou/gopsutil/cpu"
	"github.com/sirupsen/logrus"
)

var (
	port     string
	hostName string
)

func environmentHandler(w http.ResponseWriter, r *http.Request) {
	var variables map[string]string
	variables = make(map[string]string)

	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		variables[pair[0]] = pair[1]
	}

	js, err := json.Marshal(variables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	etag := fmt.Sprintf("%x", crc32.ChecksumIEEE(js))

	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("ETag", etag)
	w.Write(js)
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	var variables map[string]string

	variables = make(map[string]string)
	variables["url"] = r.URL.String()
	variables["proto"] = r.Proto
	variables["host"] = r.Host
	variables["method"] = r.Method
	variables["request-uri"] = r.RequestURI
	variables["remote-addr"] = r.RemoteAddr

	js, err := json.Marshal(variables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

var healthy = true

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if healthy {
		fmt.Fprintf(w, "ok")
	} else {
		http.Error(w, "service toggled to unhealthy", http.StatusServiceUnavailable)
	}
}

var ready = true

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if ready {
		fmt.Fprintf(w, "service is ready")
	} else {
		http.Error(w, "service is not ready", http.StatusServiceUnavailable)
	}
}

var quitLoad = make(chan bool, 1)

func burnCPU() {
	for {
		select {
		case <-quitLoad:
			return
		default:
		}
	}
}

func increaseCpuLoadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	go burnCPU()
	fmt.Fprintf(w, "CPU load increased\n")
}

func decreaseCpuLoadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	select {
	case quitLoad <- true:
		fmt.Fprintf(w, "CPU load decreased\n")
	default:
		fmt.Fprintf(w, "no additional CPU load decreased\n")
	}
}

func toggleHealthHandler(w http.ResponseWriter, r *http.Request) {
	healthy = !healthy
	fmt.Fprintf(w, "toggled health to %v", healthy)
}

func toggleReadyHandler(w http.ResponseWriter, r *http.Request) {
	ready = !ready
	fmt.Fprintf(w, "toggled ready to %v", ready)
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	ready = false
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = fmt.Fprintf(w, "signaled SIGTERM to %d", p.Pid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	healthy = false
}

func headerHandler(w http.ResponseWriter, r *http.Request) {
	hdr := r.Header
	hdr["Host"] = []string{r.Host}
	hdr["Content-Type"] = []string{r.Header.Get("Content-Type")}
	hdr["User-Agent"] = []string{r.UserAgent()}

	js, err := json.Marshal(hdr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func cpuInfoHandler(w http.ResponseWriter, r *http.Request) {
	js, err := json.Marshal(cpus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func notServing(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/index.html", http.StatusMovedPermanently)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "sorry, paas-monitor is not serving static requests.\n")
	}
}

var (
	count = 0
	cpus  InfoStatArray
)

type InfoStatArray []cpu.InfoStat

func (a InfoStatArray) Len() int {
	return len(a)
}

func (a InfoStatArray) Less(i, j int) bool {
	return strings.Compare(a[i].PhysicalID, a[j].PhysicalID) == -1
}

func (a InfoStatArray) Swap(i, j int) {
	old := a[i]
	a[i] = a[j]
	a[j] = old
}

func getCpuInfo() {
	var err error
	cpus, err = cpu.Info()
	if err != nil {
		log.Printf("failed to get cpu info, %s", err)
		cpus = make([]cpu.InfoStat, 0)
	}
	sort.Sort(cpus)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	variables := make(map[string]interface{})

	release := os.Getenv("RELEASE")
	message := os.Getenv("MESSAGE")
	if message == "" {
		message = "Hello World"
	} else {
		message = os.ExpandEnv(message)
	}
	count = count + 1

	variables["key"] = fmt.Sprintf("%s:%s", hostName, port)
	variables["release"] = release
	variables["servercount"] = fmt.Sprintf("%d", count)
	variables["message"] = message
	variables["healthy"] = healthy
	variables["ready"] = ready
	variables["cpu"] = nil

	percentage, err := cpu.Percent(0, true)
	if err == nil {
		total := 0
		for i := 0; i < len(percentage); i++ {
			total = total + int(percentage[i])
		}
		variables["cpu"] = int(total / len(percentage))
	}

	variables["cpu_id"] = ""
	if len(cpus) > 0 {
		variables["cpu_id"] = cpus[0].PhysicalID
	}

	js, err := json.Marshal(variables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Connection", "close")
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	w.Write(js)
}

func newUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

// newPAASMonitorHandler creates an http.Handler with all routes registered.
func newPAASMonitorHandler(fs http.Handler, noStaticContent bool) http.Handler {
	mux := http.NewServeMux()

	if !noStaticContent {
		mux.Handle("/", fs)
	} else {
		mux.HandleFunc("/", notServing)
	}

	mux.HandleFunc("/environment", environmentHandler)
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/header", headerHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.HandleFunc("/toggle-health", toggleHealthHandler)
	mux.HandleFunc("/toggle-ready", toggleReadyHandler)
	mux.HandleFunc("/request", requestHandler)
	mux.HandleFunc("/stop", stopHandler)
	mux.HandleFunc("/increase-cpu", increaseCpuLoadHandler)
	mux.HandleFunc("/decrease-cpu", decreaseCpuLoadHandler)
	mux.HandleFunc("/cpus", cpuInfoHandler)

	return mux
}

func main() {
	var dir string
	dir = os.Getenv("APPDIR")
	if dir == "" {
		dir = "."
	}

	fs := http.FileServer(http.Dir(dir + "/public"))

	hostName, _ = os.Hostname()
	if os.Getenv("K_REVISION") != "" {
		uuid, err := newUUID()
		if err != nil {
			log.Fatal("failed to generate uuid, %s", err)
		}
		// if running in Cloud Run use a random hostName, otherwise it is localhost
		hostName = fmt.Sprintf("%s-%s", os.Getenv("K_REVISION"), uuid)
	}

	portEnvName := flag.String("port-env-name", "", "the environment variable name overriding the listen port")
	portSpecified := flag.String("port", "", "the port to listen, override the environment name")
	healthCheck := flag.Bool("check", false, "check whether the service is listening")
	noStaticContent := flag.Bool("no-static-content", false, "only, service dynamic requests")
	zitiServerConfiguration := flag.String("ziti-server-configuration", "", "the ziti server configuration file")
	flag.Parse()

	getCpuInfo()

	// Build a handler and mount it at "/" so it works with the default mux as well.
	handler := newPAASMonitorHandler(fs, noStaticContent != nil && *noStaticContent)

	if *portSpecified != "" {
		port = *portSpecified
	} else {
		if *portEnvName != "" && os.Getenv(*portEnvName) != "" {
			port = os.Getenv(*portEnvName)
		} else {
			if *portEnvName != "" {
				log.Fatalf("environment variable '%s' is not set with a port override\n", *portEnvName)
			}
			port = os.Getenv("PORT")
			if port == "" {
				port = "1337"
			}
		}
	}

	if zitiServerConfiguration != nil && *zitiServerConfiguration != "" {
		os.Exit(zitifiedServer(*zitiServerConfiguration, handler))
	}
	if *healthCheck {
		resp, err := http.Get("http://0.0.0.0:" + port + "/health")
		if err != nil {
			log.Fatal(fmt.Errorf("%s", err))
		}
		if resp.StatusCode == 200 {
			os.Exit(0)
		} else {
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				fmt.Printf("%s\n", body)
			}
			log.Fatal(fmt.Errorf("expected status code 200, got %d", resp.StatusCode))
		}
	} else {
		srv := http.Server{
			Addr:    ":" + port,
			Handler: handler, // use our handler
		}

		idleConnsClosed := make(chan struct{})
		go func() {
			sigint := make(chan os.Signal, 1)

			// interrupt signal sent from terminal
			signal.Notify(sigint, os.Interrupt)
			// sigterm signal sent from kubernetes
			signal.Notify(sigint, syscall.SIGTERM)

			<-sigint

			// We received an interrupt signal, shut down.
			log.Printf("received signal.")
			if err := srv.Shutdown(context.Background()); err != nil {
				// Error from closing listeners, or context timeout:
				log.Printf("HTTP server shutdown failed: %v", err)
			}
			close(idleConnsClosed)
		}()

		log.Printf("listening on port %s\n", port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			log.Printf("HTTP server failed, %v", err)
		}

		<-idleConnsClosed
	}
}

func zitifiedServer(configurationFile string, handler http.Handler) int {

	if os.Getenv("DEBUG") == "true" {
		pfxlog.GlobalInit(logrus.DebugLevel, pfxlog.DefaultOptions())
		pfxlog.Logger().Debugf("debug enabled")
	}

	cfg, err := ziti.NewConfigFromFile(configurationFile)
	if err != nil {
		panic(err)
	}

	ctx, err := ziti.NewContext(cfg)
	if err != nil {
		panic(err)
	}

	serviceName := "paas-monitor"
	listener, err := ctx.ListenWithOptions(serviceName, &ziti.ListenOptions{
		ConnectTimeout: 5 * time.Minute,
		MaxTerminators: 3,
	})
	if err != nil {
		fmt.Printf("Error binding service %+v\n", err)
		panic(err)
	}

	fmt.Printf("listening for requests for Ziti service %v\n", serviceName)
	if err = http.Serve(listener, handler); err != nil {
		fmt.Sprintf("ziti stopped serving %s", err)
		return 1
	}
	return 0
}
