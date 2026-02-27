package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/edvin/hosting/internal/setup"
)

//go:embed dist/*
var staticFiles embed.FS

func main() {
	addr := flag.String("addr", ":8400", "Listen address")
	outputDir := flag.String("output", ".", "Output directory for generated files")
	noBrowser := flag.Bool("no-browser", false, "Don't open browser automatically")
	flag.Parse()

	// Strip the "dist/" prefix so files are served from root
	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		log.Fatalf("static files: %v", err)
	}

	srv := setup.NewServer(*outputDir, staticFS)

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	url := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
	fmt.Printf("Setup wizard running at %s\n", url)

	if !*noBrowser {
		openBrowser(url)
	}

	if err := http.Serve(listener, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}
