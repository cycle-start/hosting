package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/edvin/hosting/internal/setup"
)

//go:embed dist/*
var staticFiles embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "generate" {
		runGenerate(os.Args[2:])
		return
	}

	runWizard(os.Args[1:])
}

func runWizard(args []string) {
	flags := flag.NewFlagSet("setup", flag.ExitOnError)
	host := flags.String("host", "localhost", "Bind address (use 0.0.0.0 for remote access)")
	port := flags.String("port", "8400", "Listen port")
	outputDir := flags.String("output", ".", "Output directory for generated files")
	noBrowser := flags.Bool("no-browser", false, "Don't open browser automatically")
	flags.Parse(args)

	// Strip the "dist/" prefix so files are served from root
	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		log.Fatalf("static files: %v", err)
	}

	srv := setup.NewServer(*outputDir, staticFS)

	addr := net.JoinHostPort(*host, *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	listenPort := listener.Addr().(*net.TCPAddr).Port
	displayHost := *host
	if displayHost == "" || displayHost == "0.0.0.0" || displayHost == "::" {
		displayHost = "0.0.0.0"
		fmt.Printf("Setup wizard listening on %s:%d\n", displayHost, listenPort)
		fmt.Printf("  Local:   http://localhost:%d\n", listenPort)
	} else {
		fmt.Printf("Setup wizard running at http://%s:%d\n", displayHost, listenPort)
	}
	fmt.Printf("\n  Tip: To access from another machine, use SSH port forwarding:\n")
	fmt.Printf("    ssh -L %d:localhost:%d <user>@<this-host>\n", listenPort, listenPort)
	fmt.Printf("  Then open http://localhost:%d in your browser.\n\n", listenPort)

	if !*noBrowser && (*host == "localhost" || *host == "127.0.0.1") {
		openBrowser(fmt.Sprintf("http://localhost:%d", listenPort))
	}

	if err := http.Serve(listener, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runGenerate(args []string) {
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	manifestPath := flags.String("f", "setup.yaml", "Path to setup manifest file")
	outputDir := flags.String("output", ".", "Output directory for generated files")
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: setup generate [-f setup.yaml] [-output .]\n\n")
		fmt.Fprintf(os.Stderr, "Generate deployment files from a setup manifest.\n\n")
		flags.PrintDefaults()
	}
	flags.Parse(args)

	fmt.Printf("Generating deployment files from %s...\n", *manifestPath)
	if err := setup.GenerateFromManifest(*manifestPath, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done.")
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
