package main

import (
    "os"
    "fmt"
    "strings"
    "os/exec"
    "net/http"
    "encoding/json"
)

var hostName string


func determineHostname() {
    cmd := exec.Command("hostname")
    stdout, err := cmd.Output()

    if err != nil {
        println(err.Error())
        return
    }
    hostName = strings.TrimSpace(string(stdout))
    print(hostName)
}

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

    
    port := os.Getenv("PORT")
    release := os.Getenv("RELEASE")

    variables = make(map[string]string)
    variables["result"] = fmt.Sprintf("%s:%s", hostName, port)
    variables["release"] = release
    variables["message"] = fmt.Sprintf("Hello world from %s", release)

    js, err := json.Marshal(variables)
    if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write(js)
}

func main() {
    fs := http.FileServer(http.Dir("public"))
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

    determineHostname()
    http.ListenAndServe(addr, nil)
}
