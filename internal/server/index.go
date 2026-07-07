package server

import (
	"fmt"
	"net/http"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

// indexHTML is the pre-UI placeholder page. The full web UI replaces this
// in the web milestone.
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
    <p class="status">api online : ui arriving soon</p>
  </main>
</body>
</html>
`
