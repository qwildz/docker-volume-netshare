package drivers

import (
	"os"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	log "github.com/sirupsen/logrus"
)

const (
	CephOptions = "cephopts"
)

type cephDriver struct {
	volumeDriver
	username   string
	password   string
	context    string
	cephmount  string
	cephport   string
	localmount string
	cephopts   map[string]string
}

func NewCephDriver(root string, username string, password string, context string, cephmount string, cephport string, localmount string, cephopts string, mounts *MountManager) cephDriver {
	d := cephDriver{
		volumeDriver: newVolumeDriver(root, mounts),
		username:     username,
		password:     password,
		context:      context,
		cephmount:    cephmount,
		cephport:     cephport,
		localmount:   localmount,
		cephopts:     map[string]string{},
	}
	if len(cephopts) > 0 {
		d.cephopts[CephOptions] = cephopts
	}

	return d
}

func (n cephDriver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	log.Debugf("Entering Mount: %v", r)
	n.m.Lock()
	defer n.m.Unlock()
	hostdir := mountpoint(n.root, r.Name)
	source := n.fixSource(r.Name, r.ID)
	if n.mountm.HasMount(r.Name) && n.mountm.Count(r.Name) > 0 {
		log.Infof("Using existing CEPH volume mount: %s", hostdir)
		n.mountm.Increment(r.Name)
		if isMounted(hostdir) {
			return &volume.MountResponse{Mountpoint: hostdir}, nil
		}
		log.Infof("Existing CEPH volume not mounted, force remount.")
		n.mountm.Decrement(r.Name)
	}

	log.Infof("Mounting CEPH volume %s on %s", source, hostdir)
	if err := createDest(hostdir); err != nil {
		return nil, err
	}

	if err := n.mountVolume(r.Name, source, hostdir); err != nil {
		os.Remove(hostdir)
		return nil, err
	}
	n.mountm.Add(r.Name, hostdir)
	return &volume.MountResponse{Mountpoint: hostdir}, nil
}

func (n cephDriver) Unmount(r *volume.UnmountRequest) error {
	log.Debugf("Entering Unmount: %v", r)

	n.m.Lock()
	defer n.m.Unlock()
	hostdir := mountpoint(n.root, r.Name)

	if n.mountm.HasMount(r.Name) {
		if n.mountm.Count(r.Name) > 1 {
			log.Printf("Skipping unmount for %s - in use by other containers", r.Name)
			n.mountm.Decrement(r.Name)
			return nil
		}
		n.mountm.Decrement(r.Name)
	}

	log.Infof("Unmounting volume name %s from %s", r.Name, hostdir)

	if err := runUmount(hostdir); err != nil {
		return err
	}

	n.mountm.DeleteIfNotManaged(r.Name)

	if err := os.Remove(hostdir); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to remove mountpoint directory %s: %v", hostdir, err)
	}

	return nil
}

func (n cephDriver) fixSource(name, id string) string {
	if n.mountm.HasOption(name, ShareOpt) {
		return n.mountm.GetOption(name, ShareOpt)
	}
	source := strings.Split(name, "/")
	source[0] = source[0] + ":" + n.cephport + ":"
	return strings.Join(source, "/")
}

func (n cephDriver) mountVolume(name, source, dest string) error {
	options := n.mountOptions(n.mountm.GetOptions(name))

	var optParts []string
	optParts = append(optParts, n.context, n.username, n.password)

	if val, ok := options[CephOptions]; ok {
		optParts = append(optParts, val)
	}

	opts := strings.Join(optParts, ",")

	var extraArgs []string
	if log.GetLevel() == log.DebugLevel {
		extraArgs = append(extraArgs, "-v")
	}

	log.Debugf("exec: mount -t ceph %s -o %s %s", source, strings.Replace(opts, n.password, "****", 1), dest)
	return runMount("ceph", opts, source, dest, extraArgs...)
}

func (n cephDriver) mountOptions(src map[string]string) map[string]string {
	if len(n.cephopts) == 0 && len(src) == 0 {
		return EmptyMap
	}

	dst := map[string]string{}
	for k, v := range n.cephopts {
		dst[k] = v
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
