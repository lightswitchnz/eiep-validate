package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/lightswitchnz/eiep-validate/eiep"
)

//go:embed web.html
var page []byte

type webResult struct {
	Format   string         `json:"format"`
	Summary  string         `json:"summary"`
	Findings []eiep.Finding `json:"findings"`
	Warnings []eiep.Finding `json:"warnings"`
}

// serve runs the web UI on localhost and opens it in the browser.
func serve() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	http.HandleFunc("POST /check", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res := check(data, r.URL.Query().Get("strict") != "")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(webResult{res.Format, res.Summary, res.Findings, res.Warnings})
	})
	url := "http://" + ln.Addr().String()
	fmt.Printf("eiep-validate is running at %s\nClose this window to quit.\n", url)
	openBrowser(url)
	fmt.Fprintln(os.Stderr, http.Serve(ln, nil))
	os.Exit(2)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
