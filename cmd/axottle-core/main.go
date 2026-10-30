package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/axottle/axottle/internal/core"
	"github.com/axottle/axottle/internal/coreapi"
	"github.com/axottle/axottle/internal/coreui"
	"github.com/axottle/axottle/internal/enginerun"
	"github.com/axottle/axottle/internal/version"
)

func main() {
	if err := run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() > 0 {
			os.Exit(exit.ExitCode())
		}
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(version.Current())
		return nil
	}
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h" || os.Args[1] == "help") {
		fmt.Println("Usage: axottle serve | version | migrate | status | doctor | tui | logs | admin")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	engine, err := enginePath()
	if err != nil {
		return err
	}
	if len(os.Args) != 2 || os.Args[1] != "serve" {
		if engine == "" {
			return fmt.Errorf("management command requires the separately distributed axottle-engine; use axottle serve for the standalone core")
		}
		return delegate(ctx, engine, os.Args[1:])
	}
	if engine == "" && version.EngineRequired == "true" {
		return fmt.Errorf("this paired distribution requires axottle-engine alongside the core binary")
	}
	socket := os.Getenv("AXOTTLE_ENGINE_SOCKET")
	if engine != "" && socket == "" {
		data := os.Getenv("AXOTTLE_DATA_DIR")
		if data == "" {
			data = "./data"
		}
		socket, err = filepath.Abs(filepath.Join(data, "run", "engine.sock"))
		if err != nil {
			return err
		}
	}
	var child *enginerun.Process
	if engine != "" {
		if err := os.MkdirAll(filepath.Dir(socket), 0700); err != nil {
			return fmt.Errorf("prepare engine runtime directory: %w", err)
		}
		child, err = enginerun.Start(ctx, engine, version.Current(), socket, os.Stderr)
		if err != nil {
			return err
		}
		defer func() {
			shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := child.Stop(shutdown); err != nil {
				log.Print(err)
			}
		}()
	}
	trusted := "127.0.0.1/32,::1/128"
	if value, set := os.LookupEnv("AXOTTLE_TRUSTED_PROXIES"); set {
		trusted = value
	}
	var proxies []string
	if trusted != "" {
		proxies = strings.Split(trusted, ",")
	}
	coreConfig := core.Config{Version: version.Current(), EngineSocket: socket, TrustedProxies: proxies}
	if socket == "" {
		databaseURL := os.Getenv("AXOTTLE_DB_URL")
		if databaseURL == "" {
			databaseURL = "postgres://axottle:axottle_dev@localhost:5432/axottle?sslmode=disable"
		}
		key := os.Getenv("AXOTTLE_SECRET_KEY")
		const devKey = "dev-secret-change-me-in-production"
		if key == "" {
			key = devKey
		}
		var legacy []string
		if key != devKey {
			legacy = []string{devKey}
		}
		app, err := coreapi.Open(ctx, coreapi.Config{DatabaseURL: databaseURL, SecretKey: key, LegacySecretKeys: legacy, PublicURL: os.Getenv("AXOTTLE_PUBLIC_URL"), DataDir: standaloneDataDir(), UI: coreui.Handler()})
		if err != nil {
			return err
		}
		defer app.Close()
		coreConfig.Application, coreConfig.Readiness = app, app.Ready
	}
	handler, err := core.New(coreConfig)
	if err != nil {
		return err
	}
	defer handler.Close()
	host, port := os.Getenv("AXOTTLE_HOST"), os.Getenv("AXOTTLE_PORT")
	if host == "" {
		host = "0.0.0.0"
	}
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: net.JoinHostPort(host, port), Handler: handler,
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.ListenAndServe() }()
	log.Printf("Axottle core %s listening on %s", version.Current(), server.Addr)
	var engineDone <-chan struct{}
	if child != nil {
		engineDone = child.Done()
	}
	var result error
	select {
	case <-ctx.Done():
	case err := <-serveResult:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
	case <-engineDone:
		result = fmt.Errorf("engine stopped unexpectedly: %v", child.Err())
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Close public requests and the engine concurrently so active streams can
	// finish when the engine cancels its own request contexts.
	stopped := make(chan error, 1)
	go func() {
		if child != nil {
			stopped <- child.Stop(shutdown)
		} else {
			stopped <- nil
		}
	}()
	_ = server.Shutdown(shutdown)
	if err := <-stopped; err != nil && result == nil {
		result = err
	}
	return result
}

func enginePath() (string, error) {
	if path, set := os.LookupEnv("AXOTTLE_ENGINE_PATH"); set {
		return path, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Historical upgraders already replace install_static atomically with the
	// release tree. Shipping the engine there supports their first paired
	// upgrade without a network bootstrap or changes to an installed script.
	for _, path := range []string{
		filepath.Join(filepath.Dir(executable), "..", "install_static", "axottle-engine"),
		filepath.Join(filepath.Dir(executable), "axottle-engine"),
	} {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return "", err
		}
		return filepath.Clean(path), nil
	}
	return "", nil
}

func delegate(ctx context.Context, binary string, args []string) error {
	cmd := exec.Command(binary, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-done:
			return err
		case <-time.After(30 * time.Second):
			_ = cmd.Process.Kill()
			return <-done
		}
	}
}

func standaloneDataDir() string {
	if value := os.Getenv("AXOTTLE_DATA_DIR"); value != "" {
		return value
	}
	return "./data"
}
