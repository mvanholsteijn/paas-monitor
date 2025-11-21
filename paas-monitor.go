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

	"cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
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
		_, err := fmt.Fprint(w, "service is ready")
		if err != nil {
			log.Printf("failed to write to response, %s", err)
		}
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

func increaseCPULoadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	go burnCPU()
	_, err := fmt.Fprint(w, "CPU load increased\n")
	if err != nil {
		log.Printf("failed to write to response, %s", err)
	}
}

func decreaseCPULoadHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	select {
	case quitLoad <- true:
		_, err = fmt.Fprint(w, "CPU load decreased\n")
	default:
		_, err = fmt.Fprint(w, "no additional CPU load decreased\n")
	}
	if err != nil {
		log.Printf("failed to write to response, %s", err)
	}
}

func toggleHealthHandler(w http.ResponseWriter, r *http.Request) {
	healthy = !healthy
	_, err := fmt.Fprintf(w, "toggled health to %v", healthy)
	if err != nil {
		log.Printf("failed to write to response, %s", err)
	}
}

func toggleReadyHandler(w http.ResponseWriter, r *http.Request) {
	ready = !ready
	_, err := fmt.Fprintf(w, "toggled ready to %v", ready)
	if err != nil {
		log.Printf("failed to write to response, %s", err)
	}
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
		log.Printf("failed to write to response, %s", err)
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
		_, err := fmt.Fprint(w, "sorry, paas-monitor is not serving static requests.\n")
		if err != nil {
			log.Printf("failed to write to response, %s", err)
		}
	}
}

var (
	count = 0
	cpus  cpuInfos
)

type cpuInfos []cpu.InfoStat

func (a cpuInfos) Len() int {
	return len(a)
}

func (a cpuInfos) Less(i, j int) bool {
	return strings.Compare(a[i].PhysicalID, a[j].PhysicalID) == -1
}

func (a cpuInfos) Swap(i, j int) {
	old := a[i]
	a[i] = a[j]
	a[j] = old
}

func getCPUInfo() {
	var err error
	cpus, err = cpu.Info()
	if err != nil {
		log.Printf("failed to get cpu info, %s", err)
		cpus = make([]cpu.InfoStat, 0)
	}
	sort.Sort(cpus)
}

func createStatusHandler(keepConnection bool) func(w http.ResponseWriter, r *http.Request) {

	statusHandler := func(w http.ResponseWriter, r *http.Request) {
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
		if !keepConnection {
			w.Header().Set("Connection", "close")
		}
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if _, err = w.Write(js); err != nil {
			log.Printf("failed to write to response, %s", err)
		}
	}

	return statusHandler
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
func newPAASMonitorHandler(fs http.Handler, noStaticContent bool, keepConnection bool) http.Handler {
	mux := http.NewServeMux()

	if !noStaticContent {
		mux.Handle("/", fs)
	} else {
		mux.HandleFunc("/", notServing)
	}

	mux.HandleFunc("/environment", environmentHandler)
	mux.HandleFunc("/status", createStatusHandler(keepConnection))
	mux.HandleFunc("/header", headerHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.HandleFunc("/toggle-health", toggleHealthHandler)
	mux.HandleFunc("/toggle-ready", toggleReadyHandler)
	mux.HandleFunc("/request", requestHandler)
	mux.HandleFunc("/stop", stopHandler)
	mux.HandleFunc("/increase-cpu", increaseCPULoadHandler)
	mux.HandleFunc("/decrease-cpu", decreaseCPULoadHandler)
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
			log.Fatalf("failed to generate uuid, %s", err)
		}
		// if running in Cloud Run use a random hostName, otherwise it is localhost
		hostName = fmt.Sprintf("%s-%s", os.Getenv("K_REVISION"), uuid)
	}

	portEnvName := flag.String("port-env-name", "", "the environment variable name overriding the listen port")
	portSpecified := flag.String("port", "", "the port to listen, override the environment name")
	healthCheck := flag.Bool("check", false, "check whether the service is listening")
	keepConnection := flag.Bool("keep-connection", false, "do not add connection close header on each response")
	noStaticContent := flag.Bool("no-static-content", false, "only, service dynamic requests")
	zitiServerConfiguration := flag.String("ziti-server-configuration", "", "a file or google secret manager gsm:///projects/<>/secrets/<>/versions/latest")
	flag.Parse()

	getCPUInfo()

	// Build a handler and mount it at "/" so it works with the default mux as well.
	handler := newPAASMonitorHandler(fs, noStaticContent != nil && *noStaticContent, *keepConnection)

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

	if *healthCheck {
		resp, err := http.Get("http://0.0.0.0:" + port + "/health")
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode == 200 {
			os.Exit(0)
		} else {
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				log.Printf("%s\n", body)
			}
			log.Fatal(fmt.Errorf("expected status code 200, got %d", resp.StatusCode))
		}
	}

	if zitiServerConfiguration != nil && *zitiServerConfiguration != "" {
		os.Exit(zitifiedServer(*zitiServerConfiguration, handler))
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

func getSecret(name string) ([]byte, error) {
	ctx := context.Background()

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient: %w", err)
	}

	defer func(client *secretmanager.Client) {
		err := client.Close()
		if err != nil {

		}
	}(client)

	response, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return nil, fmt.Errorf("access secret %q: %w", name, err)
	}
	if response.Payload == nil {
		return nil, fmt.Errorf("secret %q has empty payload", name)
	}
	data := response.Payload.Data
	if response.Payload.DataCrc32C != nil {
		sum := crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))
		if sum != uint32(*response.Payload.DataCrc32C) {
			return nil, fmt.Errorf("secret payload checksum mismatch (possible data corruption)")
		}
	}

	return data, nil
}

func zitifiedServer(configurationFile string, handler http.Handler) int {

	if os.Getenv("DEBUG") == "true" {
		pfxlog.GlobalInit(logrus.DebugLevel, pfxlog.DefaultOptions())
		pfxlog.Logger().Debugf("debug enabled")
	}

	var cfg *ziti.Config
	if strings.HasPrefix(configurationFile, "gsm:///") {
		secretName := strings.TrimPrefix(configurationFile, "gsm:///")
		secret, err := getSecret(secretName)
		if err != nil {
			log.Fatal(err)
		}
		cfg = &ziti.Config{}
		err = json.Unmarshal(secret, cfg)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		var err error
		cfg, err = ziti.NewConfigFromFile(configurationFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	ctx, err := ziti.NewContext(cfg)
	if err != nil {
		log.Fatal(err)
	}

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	signal.Notify(sigint, os.Interrupt, syscall.SIGINT)

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", healthHandler)

	healthSrv := &http.Server{
		Addr:    ":1337",
		Handler: healthMux,
	}

	zitiSrv := &http.Server{Handler: handler}
	if err != nil {
		log.Printf("Error binding service %+v\n", err)
		log.Fatal(err)
	}

	go func() {
		log.Printf("starting health check server on port 1337")
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("health check server failed: %v", err)
		}
	}()

	done := make(chan bool)

	go func() {
		defer close(done)
		serviceName := "paas-monitor"
		log.Printf("listening for requests for Ziti service %v\n", serviceName)
		zitiListener, err := ctx.ListenWithOptions(serviceName, &ziti.ListenOptions{
			ConnectTimeout: 5 * time.Minute,
			MaxTerminators: 3,
		})
		if err != nil {
			log.Printf("failed to listen for Ziti service %v: %v", serviceName, err)
			return
		}
		if err = zitiSrv.Serve(zitiListener); err != nil {
			log.Printf("ziti stopped serving %s", err)
		}
	}()

	select {
	case <-sigint:
		log.Printf("received signal, shutting down servers...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("health server shutdown failed: %v", err)
		}

		if err := zitiSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("ziti server shutdown failed: %v", err)
		}
		select {
		case <-done:
			log.Printf("servers shut down gracefully")
		case <-shutdownCtx.Done():
			log.Printf("servers failed to shutdown in 30s")
			return 1
		}

	case <-done:
		log.Printf("ziti server stopped")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("health server shutdown failed: %v", err)
		}
	}

	return 0
}
