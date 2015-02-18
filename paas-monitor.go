package main

import (
    "os"
    "fmt"
    "strings"
    "net/http"
    "encoding/json"
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

    w.Header().Set("Content-Type", "application/json")
    w.Write(js)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
    var variables map[string]string

    hostName, _ := os.Hostname()
    port := os.Getenv("PORT")
    release := os.Getenv("RELEASE")

    variables = make(map[string]string)
    variables["key"] = fmt.Sprintf("%s:%s", hostName, port)
    variables["release"] = release
    variables["message"] = fmt.Sprintf("Hello World from %s", release)

    js, err := json.Marshal(variables)
    if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
    }
    w.Header().Set("Content-Type", "application/json")
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


    var addr string
    port := os.Getenv("PORT")
    if port != "" {
	addr = ":" + port
    } else {
	addr = ":1337"
	os.Setenv("PORT", "1337")
    }

    http.ListenAndServe(addr, nil)
}
