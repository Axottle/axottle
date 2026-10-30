package enginebridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type lockedListener struct {
	net.Listener
	lock *os.File
}

func (l *lockedListener) Close() error {
	err := l.Listener.Close()
	// Closing the descriptor releases the advisory lock, including after a
	// process crash. Keep the lock file itself to avoid an unlink/relock race.
	_ = l.lock.Close()
	return err
}

// ListenUnix holds a process lock for the listener's lifetime so that a crashed
// engine's stale socket can be reclaimed while a running engine stays intact.
// Regular files and symlinks are never replaced.
func ListenUnix(path string) (net.Listener, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("engine socket must be an absolute path")
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, fmt.Errorf("open engine socket lock: %w", err)
	}
	locked := false
	defer func() {
		if !locked {
			_ = lock.Close()
		}
	}()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, fmt.Errorf("engine socket is already owned: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("engine socket path is not a socket")
		}
		conn, dialErr := net.DialTimeout("unix", path, time.Second)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("engine socket is already accepting connections")
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) && !errors.Is(dialErr, os.ErrNotExist) {
			return nil, fmt.Errorf("cannot establish that engine socket is stale: %w", dialErr)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on engine socket: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("restrict engine socket permissions: %w", err)
	}
	locked = true
	return &lockedListener{Listener: listener, lock: lock}, nil
}

type Client struct {
	transport *http.Transport
	identity  *http.Client
}

func NewClient(socket string) (*Client, error) {
	if !filepath.IsAbs(socket) {
		return nil, fmt.Errorf("engine socket must be an absolute path")
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", socket)
		},
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       60 * time.Second,
		ResponseHeaderTimeout: 90 * time.Second,
	}
	return &Client{transport: transport, identity: &http.Client{
		Transport:     transport,
		Timeout:       2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) Close() { c.transport.CloseIdleConnections() }

func (c *Client) Identity(ctx context.Context) (Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://engine"+IdentityPath, nil)
	if err != nil {
		return Identity{}, err
	}
	resp, err := c.identity.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("engine is unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("engine identity returned HTTP %d", resp.StatusCode)
	}
	var id Identity
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&id); err != nil {
		return Identity{}, fmt.Errorf("decode engine identity: %w", err)
	}
	return id, nil
}
