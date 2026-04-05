package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"netFlow_tool-ui/service"
	"netFlow_tool-ui/types"
)

//go:embed webui/dist/**
var webAssets embed.FS

const websocketMagic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	throughputChartWindow   = 2 * time.Minute
	throughputChartMaxPoint = 15
	webTreeAverageWindow    = 2 * time.Minute
)

type Server struct {
	statsSvc *service.StatsService
	server   *http.Server
	listener net.Listener
	url      string
}

type bootstrapResponse struct {
	Flows      []types.ProcessFlow     `json:"flows"`
	History    []types.DailyUsage      `json:"history"`
	Throughput []types.ThroughputPoint `json:"throughput"`
}

type wsSnapshot struct {
	Type       string                  `json:"type"`
	Flows      []types.ProcessFlow     `json:"flows"`
	Throughput []types.ThroughputPoint `json:"throughput"`
	Timestamp  string                  `json:"timestamp"`
}

func Start(statsSvc *service.StatsService) (*Server, string, error) {
	listener, err := listenRandomHighPort()
	if err != nil {
		return nil, "", err
	}

	mux := http.NewServeMux()
	server := &Server{
		statsSvc: statsSvc,
		listener: listener,
		url:      fmt.Sprintf("http://%s", listener.Addr().String()),
	}

	distFS, err := fs.Sub(webAssets, "webui/dist")
	if err != nil {
		return nil, "", err
	}
	staticHandler, err := newStaticHandler(distFS)
	if err != nil {
		return nil, "", err
	}
	mux.HandleFunc("/api/bootstrap", server.handleBootstrap)
	mux.HandleFunc("/ws", server.handleWebSocket)
	mux.Handle("/", staticHandler)
	server.server = &http.Server{Handler: mux}

	go func() {
		if err := server.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Web UI server error: %v", err)
		}
	}()

	return server, server.url, nil
}

func newStaticHandler(distFS fs.FS) (http.Handler, error) {
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(distFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if shouldServeIndex(distFS, target) {
			http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(indexHTML))
			return
		}

		r.URL.Path = "/" + target
		fileServer.ServeHTTP(w, r)
	}), nil
}

func shouldServeIndex(distFS fs.FS, target string) bool {
	if target == "" {
		return true
	}

	if path.Ext(target) == "" {
		return true
	}

	info, err := fs.Stat(distFS, target)
	if err != nil {
		return !errors.Is(err, fs.ErrNotExist) && path.Ext(target) == ""
	}

	return info.IsDir()
}

func (s *Server) Stop() error {
	if s == nil || s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Server) handleBootstrap(w http.ResponseWriter, _ *http.Request) {
	flows := s.statsSvc.SnapshotAveragedFlows(webTreeAverageWindow)
	if len(flows) == 0 {
		var err error
		flows, err = s.statsSvc.Snapshot()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}

	history, historyErr := s.statsSvc.SnapshotHistory()
	if historyErr != nil {
		http.Error(w, historyErr.Error(), http.StatusBadGateway)
		return
	}

	throughput := s.statsSvc.SnapshotThroughput(throughputChartWindow, throughputChartMaxPoint)

	writeJSON(w, bootstrapResponse{
		Flows:      flows,
		History:    history,
		Throughput: throughput,
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !headerContainsToken(r.Header, "Connection", "Upgrade") || !headerContainsToken(r.Header, "Upgrade", "websocket") {
		http.Error(w, "upgrade required", http.StatusUpgradeRequired)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket unsupported", http.StatusInternalServerError)
		return
	}

	conn, buf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "failed to hijack websocket", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	accept := computeWebSocketAccept(key)
	var response bytes.Buffer
	response.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	response.WriteString("Upgrade: websocket\r\n")
	response.WriteString("Connection: Upgrade\r\n")
	response.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	if _, err := buf.Write(response.Bytes()); err != nil {
		return
	}
	if err := buf.Flush(); err != nil {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		flows := s.statsSvc.SnapshotAveragedFlows(webTreeAverageWindow)
		if len(flows) == 0 {
			var err error
			flows, err = s.statsSvc.Snapshot()
			if err != nil {
				return
			}
		}

		if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
			return
		}

		payload, err := json.Marshal(wsSnapshot{
			Type:       "snapshot",
			Flows:      flows,
			Throughput: s.statsSvc.SnapshotThroughput(throughputChartWindow, throughputChartMaxPoint),
			Timestamp:  time.Now().Format(time.RFC3339),
		})
		if err != nil {
			return
		}

		if err := writeWebSocketText(conn, payload); err != nil {
			return
		}

		<-ticker.C
	}
}

func listenRandomHighPort() (net.Listener, error) {
	for attempt := 0; attempt < 32; attempt++ {
		port, err := randomHighPort()
		if err != nil {
			return nil, err
		}
		listener, listenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if listenErr == nil {
			return listener, nil
		}
	}

	return nil, fmt.Errorf("failed to bind a random high port")
}

func randomHighPort() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(65535-49152))
	if err != nil {
		return 0, err
	}
	return 49152 + int(n.Int64()), nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func computeWebSocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + websocketMagic))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func headerContainsToken(header http.Header, name, expected string) bool {
	for _, value := range header.Values(name) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), expected) {
				return true
			}
		}
	}
	return false
}

func writeWebSocketText(w io.Writer, payload []byte) error {
	frame := make([]byte, 0, len(payload)+10)
	frame = append(frame, 0x81)

	payloadLen := len(payload)
	switch {
	case payloadLen <= 125:
		frame = append(frame, byte(payloadLen))
	case payloadLen <= 65535:
		frame = append(frame, 126, byte(payloadLen>>8), byte(payloadLen))
	default:
		frame = append(frame, 127, 0, 0, 0, 0, byte(payloadLen>>24), byte(payloadLen>>16), byte(payloadLen>>8), byte(payloadLen))
	}

	frame = append(frame, payload...)
	_, err := w.Write(frame)
	return err
}
