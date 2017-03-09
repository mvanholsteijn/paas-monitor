package main // import "github.com/mvanholsteijn/paas-monitor"

import (
    "os"
    "flag"
    "fmt"
    "strings"
    "net/http"
    "encoding/json"
    "log"
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

    w.Header().Set("Content-Type", "application/json")
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    fmt.Fprintf(w, "ok")
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    fmt.Fprintf(w, "stopped on request")
    os.Exit(1)
}

func headerHandler(w http.ResponseWriter, r *http.Request) {

    hdr := r.Header
    hdr["Host"] = []string { r.Host }
    hdr["Content-Type"] = []string { r.Header.Get("Content-Type") }
    hdr["User-Agent"] = []string { r.UserAgent() }

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
    fs := http.FileServer(http.Dir( dir + "/public"))

    http.Handle("/", fs)
    http.HandleFunc("/environment", environmentHandler)
    http.HandleFunc("/status", statusHandler)
    http.HandleFunc("/header", headerHandler)
    http.HandleFunc("/health", healthHandler)
    http.HandleFunc("/request", requestHandler)
    http.HandleFunc("/stop", stopHandler)


    portEnvName := flag.String("port-env-name", "", "the environment variable name overriding the listen port")
    portSpecified := flag.String("port", "", "the port to listen, override the environment name")
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

    log.Printf("listening on port %s\n", port)
    err := http.ListenAndServe(":" + port, nil)
    log.Fatal("%s", err)
}
