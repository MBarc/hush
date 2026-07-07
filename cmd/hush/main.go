package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	switch os.Args[1] {
	case "serve":
		serve()
	case "version":
		fmt.Println("hush " + version)
	default:
		usage()
	}
}

func usage() {
	fmt.Print(`hush - a quiet little vault for your homelab

Usage:
  hush serve      start the server
  hush version    print the version

The full command set arrives with the first release.
`)
}

func serve() {
	addr := os.Getenv("HUSH_LISTEN")
	if addr == "" {
		addr = ":4874"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/", handleIndex)
	log.Printf("hush %s listening on %s", version, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","version":%q}`+"\n", version)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>hush</title>
<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Cdefs%3E%3ClinearGradient id='g' x1='0' y1='0' x2='1' y2='1'%3E%3Cstop offset='0' stop-color='%23A78BFF'/%3E%3Cstop offset='1' stop-color='%237C5CFF'/%3E%3C/linearGradient%3E%3C/defs%3E%3Cpath fill='url(%23g)' d='M20 8 h24 a14 14 0 0 1 14 14 v12 a14 14 0 0 1 -14 14 h-14 l-10 10 v-10 a14 14 0 0 1 -14 -14 v-12 a14 14 0 0 1 14 -14 z'/%3E%3Ccircle cx='22' cy='28' r='4.5' fill='%230D0B12'/%3E%3Ccircle cx='32' cy='28' r='4.5' fill='%230D0B12'/%3E%3Ccircle cx='42' cy='28' r='4.5' fill='%230D0B12'/%3E%3C/svg%3E">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    background: #0D0B12;
    color: #EDE9F8;
    font-family: Inter, system-ui, sans-serif;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .card {
    background: #151020;
    border: 1px solid #2A2140;
    border-radius: 10px;
    padding: 48px 56px;
    text-align: center;
    max-width: 440px;
  }
  .mark { width: 72px; height: 72px; margin-bottom: 20px; }
  h1 { font-size: 40px; font-weight: 700; letter-spacing: -1px; }
  .tagline { color: #A79BC4; margin-top: 8px; font-size: 15px; }
  .status {
    margin-top: 28px;
    display: inline-block;
    font-family: "JetBrains Mono", ui-monospace, monospace;
    font-size: 13px;
    color: #8F6FFF;
    background: #1D1730;
    border: 1px solid #3A2F57;
    border-radius: 6px;
    padding: 8px 14px;
  }
</style>
</head>
<body>
  <main class="card">
    <svg class="mark" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" aria-label="hush logo">
      <defs>
        <linearGradient id="hv" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stop-color="#A78BFF"/>
          <stop offset="1" stop-color="#7C5CFF"/>
        </linearGradient>
      </defs>
      <path fill="url(#hv)" d="M20 8 h24 a14 14 0 0 1 14 14 v12 a14 14 0 0 1 -14 14 h-14 l-10 10 v-10 a14 14 0 0 1 -14 -14 v-12 a14 14 0 0 1 14 -14 z"/>
      <circle cx="22" cy="28" r="4.5" fill="#0D0B12"/>
      <circle cx="32" cy="28" r="4.5" fill="#0D0B12"/>
      <circle cx="42" cy="28" r="4.5" fill="#0D0B12"/>
    </svg>
    <h1>hush</h1>
    <p class="tagline">A quiet little vault for your homelab.</p>
    <p class="status">v` + version + ` : assembling quietly</p>
  </main>
</body>
</html>
`
