package drivers

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ShareSplitIndentifer = "#"
)

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

// run executes a command using direct exec (no shell) to prevent shell injection.
// The cmd string is split on whitespace. For commands that were previously run via
// "sh -c", callers should switch to runArgs() or runMount()/runUmount().
func run(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}
	if out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput(); err != nil {
		log.Println(string(out))
		return err
	}
	return nil
}

// runArgs executes a command with explicit arguments, avoiding shell interpretation.
func runArgs(name string, args ...string) error {
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		log.Println(string(out))
		return err
	}
	return nil
}

// runMount executes a mount command with the given arguments.
func runMount(fstype string, opts string, source string, dest string, extraArgs ...string) error {
	args := []string{}
	for _, a := range extraArgs {
		if a != "" {
			args = append(args, a)
		}
	}
	args = append(args, "-t", fstype)
	if opts != "" {
		args = append(args, "-o", opts)
	}
	args = append(args, source, dest)
	return runArgs("mount", args...)
}

// runUmount executes an umount command for the given mountpoint.
func runUmount(mountpoint string) error {
	return runArgs("umount", mountpoint)
}

// isMounted checks if the given path is currently mounted.
func isMounted(path string) bool {
	return runArgs("mountpoint", "-q", path) == nil
}

// IsMountedCheck is the exported version of isMounted for use by other packages (e.g., state sync).
func IsMountedCheck(path string) bool {
	return isMounted(path)
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
