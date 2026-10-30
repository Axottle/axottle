package enginebridge

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "axunix-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

func TestSocketOwnershipAndPermissions(t *testing.T) {
	path := shortSocket(t)
	l, err := ListenUnix(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("socket mode: %v", info.Mode())
	}
	if second, err := ListenUnix(path); err == nil {
		second.Close()
		t.Fatal("second listener stole socket")
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal("original listener no longer reachable:", err)
	}
	conn.Close()
}

func TestSocketRefusesFilesAndSymlinks(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		path := shortSocket(t)
		target := path + "-target"
		if err := os.WriteFile(target, []byte("preserve"), 0600); err != nil {
			t.Fatal(err)
		}
		if symlink {
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Rename(target, path); err != nil {
				t.Fatal(err)
			}
			target = path
		}
		if l, err := ListenUnix(path); err == nil {
			l.Close()
			t.Fatal("replaced non-socket")
		}
		data, err := os.ReadFile(target)
		if err != nil || string(data) != "preserve" {
			t.Fatal("existing data changed")
		}
	}
}

func TestStaleSocketRecovery(t *testing.T) {
	path := shortSocket(t)
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	stale.Close()
	l, err := ListenUnix(path)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("clean shutdown left socket behind")
	}
	l, err = ListenUnix(path)
	if err != nil {
		t.Fatal("lock was not released:", err)
	}
	l.Close()
}

func TestIdentityAndCompatibility(t *testing.T) {
	path := shortSocket(t)
	l, err := ListenUnix(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: IdentityHandler("v1")}
	go srv.Serve(l)
	defer srv.Close()
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	id, err := client.Identity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := id.Validate("v1"); err != nil {
		t.Fatal(err)
	}
	if err := id.Validate("v2"); err == nil {
		t.Fatal("mixed release accepted")
	}
	id.Protocol++
	if err := id.Validate("v1"); err == nil {
		t.Fatal("mixed protocol accepted")
	}
	if _, err := NewClient("relative.sock"); err == nil {
		t.Fatal("relative path accepted")
	}
}
