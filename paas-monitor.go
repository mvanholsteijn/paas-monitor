package main // import "github.com/mvanholsteijn/paas-monitor"

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"hash/crc32"
)

var port string

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
	variables["method"] = r.Method
	variables["url"] = r.URL.String()
	variables["proto"] = r.Proto

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

func toggleHealthHandler(w http.ResponseWriter, r *http.Request) {
	healthy = !healthy
	fmt.Fprintf(w, "toggled health to %v", healthy)
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", "0");
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	} else {
		log.Println("Damn, no flush");
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

var (
	count = 0
)

func statusHandler(w http.ResponseWriter, r *http.Request) {
	var variables map[string]string

	hostName, _ := os.Hostname()
	release := os.Getenv("RELEASE")
	message := os.Getenv("MESSAGE")
	if message == "" {
		message = "Hello World"
	}

	count = count + 1

	variables = make(map[string]string)
	variables["key"] = fmt.Sprintf("%s:%s", hostName, port)
	variables["release"] = release
	variables["servercount"] = fmt.Sprintf("%d", count)
	variables["message"] = fmt.Sprintf("%s from release %s; server call count is %d", message, release, count)

	js, err := json.Marshal(variables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Connection", "close")
	w.Write(js)
}

func main() {
	var dir string

	dir = os.Getenv("APPDIR")
	if dir == "" {
		dir = "."
	}
	fs := http.FileServer(http.Dir(dir + "/public"))

	http.Handle("/", fs)
	http.HandleFunc("/environment", environmentHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/header", headerHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/toggle-health", toggleHealthHandler)
	http.HandleFunc("/request", requestHandler)
	http.HandleFunc("/stop", stopHandler)

	portEnvName := flag.String("port-env-name", "", "the environment variable name overriding the listen port")
	portSpecified := flag.String("port", "", "the port to listen, override the environment name")
	healthCheck := flag.Bool("check", false, "check whether the service is listening")
	flag.Parse()

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
