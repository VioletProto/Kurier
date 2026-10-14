// Controlled disposable target for user-operated browser/agent checks.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:3000", "explicit loopback or private-LAN bind address")
	flag.Parse()
	host, _, e := net.SplitHostPort(*listen)
	ip, parseErr := netip.ParseAddr(host)
	if e != nil || parseErr != nil || (!ip.IsLoopback() && !ip.IsPrivate()) || ip.IsUnspecified() {
		fmt.Println("Choose an explicit loopback or private-LAN IP and port.")
		return
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Println("Target could not bind.")
		return
	}
	var sends atomic.Int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/counts" && r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(map[string]int64{"targetSends": sends.Load()})
			return
		}
		if (r.URL.Path != "/echo" && r.URL.Path != "/ambiguous") || (r.Method != "GET" && r.Method != "POST") {
			w.WriteHeader(404)
			return
		}
		if r.ContentLength > 64*1024 {
			w.WriteHeader(413)
			return
		}
		raw, e := io.ReadAll(io.LimitReader(r.Body, 64*1024+1))
		defer clear(raw)
		if e != nil || len(raw) > 64*1024 {
			w.WriteHeader(413)
			return
		}
		sends.Add(1)
		// Close only after recording the send. The operator must inspect /counts
		// and history; a new execution requires deliberate consent/submission.
		if r.URL.Path == "/ambiguous" {
			conn, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				_ = conn.Close()
			}
			return
		}
		w.Header().Set("Set-Cookie", "disposable-check-cookie")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "authorization": r.Header.Get("Authorization"), "targetSends": sends.Load()})
	})
	fmt.Println("Disposable target ready; /echo, /ambiguous and /counts. Request inputs are never logged.")
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 64 * 1024}
	if e := server.Serve(listener); e != nil && e != http.ErrServerClosed {
		fmt.Println("Target stopped.")
	}
}
