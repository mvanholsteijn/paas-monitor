package main // import "github.com/mvanholsteijn/paas-monitor"

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"hash/crc32"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

var port string
var hostName string

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

func stopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", "0")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	} else {
		log.Println("Damn, no flush")
	}
	os.Exit(1)
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "sorry, paas-monitor is not serving static requests.\n")
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
	flag.Parse()

	getCpuInfo()
        if noStaticContent == nil || *noStaticContent == false {
	    http.Handle("/", fs)
        } else {
	    http.HandleFunc("/", notServing)
        }
	http.HandleFunc("/environment", environmentHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/header", headerHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/toggle-health", toggleHealthHandler)
	http.HandleFunc("/request", requestHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/increase-cpu", increaseCpuLoadHandler)
	http.HandleFunc("/decrease-cpu", decreaseCpuLoadHandler)
	http.HandleFunc("/cpus", cpuInfoHandler)


	if *portSpecified != "" && *portEnvName != "" {
		log.Fatalf("specify either -port or -port-env-name, but not both.\n")
	}

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
		log.Printf("listening on port %s\n", port)
		err := http.ListenAndServe(":"+port, nil)
		log.Fatal("%s", err)
	}
}
