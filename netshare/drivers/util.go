package drivers

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	ShareSplitIndentifer = "#"
)

// MountTimeout is the maximum time to wait for mount operations to complete.
// Docker has a default 2-minute timeout for plugin operations, so this should
// be slightly less to allow the plugin to return a proper error message.
var MountTimeout = 100 * time.Second

func createDest(dest string) error {
	fi, err := os.Lstat(dest)

	if os.IsNotExist(err) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if fi != nil && !fi.IsDir() {
		return fmt.Errorf("%v already exist and it's not a directory", dest)
	}
	return nil
}

// Used to support on the fly volume creation using docker run. If = is in the name we split
// and elem[1] is the volume name
func resolveName(name string) (string, map[string]string) {
	if strings.Contains(name, ShareSplitIndentifer) {
		sharevol := strings.Split(name, ShareSplitIndentifer)
		opts := map[string]string{}
		opts[ShareOpt] = sharevol[0]
		opts[CreateOpt] = "true"
		return sharevol[1], opts
	}
	return name, nil
}

func shareDefinedWithVolume(name string) bool {
	return strings.Contains(name, ShareSplitIndentifer)
}

func addShareColon(share string) string {
	if strings.Contains(share, ":") {
		return share
	}
	source := strings.Split(share, "/")
	source[0] = source[0] + ":"
	return strings.Join(source, "/")
}

func mountpoint(elem ...string) string {
	return filepath.Join(elem...)
}

func run(cmd string) error {
	return runWithTimeout(cmd, MountTimeout)
}

func runWithTimeout(cmd string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(ctx, "sh", "-c", cmd)
	out, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Command timed out after %s: %s", timeout, cmd)
			return fmt.Errorf("mount operation timed out after %s", timeout)
		}
		log.Println(string(out))
		return err
	}
	return nil
}

func merge(src, src2 map[string]string) map[string]string {
	if len(src) == 0 && len(src2) == 0 {
		return EmptyMap
	}

	dst := map[string]string{}
	for k, v := range src2 {
		dst[k] = v
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
