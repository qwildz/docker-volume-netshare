package drivers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/jdx/go-netrc"
	log "github.com/sirupsen/logrus"
)

// Constants defining driver paremeters
const (
	UsernameOpt = "username"
	PasswordOpt = "password"
	DomainOpt   = "domain"
	SecurityOpt = "security"
	FileModeOpt = "fileMode"
	DirModeOpt  = "dirMode"
	CifsOpts    = "cifsopts"
)

// CifsDriver driver structure
type CifsDriver struct {
	volumeDriver
	creds    *CifsCreds
	netrc    *netrc.Netrc
	cifsopts map[string]string
}

// CifsCreds contains Options for cifs-mount
type CifsCreds struct {
	user     string
	pass     string
	domain   string
	security string
	fileMode string
	dirMode  string
}

func (creds *CifsCreds) String() string {
	return fmt.Sprintf("creds: { user=%s,pass=****,domain=%s,security=%s, fileMode=%s, dirMode=%s}", creds.user, creds.domain, creds.security, creds.fileMode, creds.dirMode)
}

// NewCifsCredentials setting the credentials
func NewCifsCredentials(user, pass, domain, security, fileMode, dirMode string) *CifsCreds {
	return &CifsCreds{user: user, pass: pass, domain: domain, security: security, fileMode: fileMode, dirMode: dirMode}
}

// NewCIFSDriver creating the cifs driver
func NewCIFSDriver(root string, creds *CifsCreds, netrc, cifsopts string, mounts *MountManager) CifsDriver {
	d := CifsDriver{
		volumeDriver: newVolumeDriver(root, mounts),
		creds:        creds,
		netrc:        parseNetRC(netrc),
		cifsopts:     map[string]string{},
	}
	if len(cifsopts) > 0 {
		d.cifsopts[CifsOpts] = cifsopts
	}
	return d
}

func parseNetRC(path string) *netrc.Netrc {
	if n, err := netrc.Parse(filepath.Join(path, ".netrc")); err == nil {
		return n
	} else {
		log.Warnf("Error: %s", err.Error())
	}
	return nil
}

// Mount do the mounting
func (c CifsDriver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	c.m.Lock()
	defer c.m.Unlock()

	resolvedName, resOpts := resolveName(r.Name)
	hostdir := mountpoint(c.root, resolvedName)
	source := c.fixSource(resolvedName)
	host := c.parseHost(resolvedName)

	log.Infof("Mount: %s, ID: %s, resolved: %s", r.Name, r.ID, resolvedName)

	// Support adhoc mounts (outside of docker volume create)
	// need to adjust source for ShareOpt
	if resOpts != nil {
		if share, found := resOpts[ShareOpt]; found {
			source = c.fixSource(share)
			host = c.parseHost(share)
		}
	}

	if c.mountm.HasMount(resolvedName) && c.mountm.Count(resolvedName) > 0 {
		log.Infof("Using existing CIFS volume mount: %s", hostdir)
		c.mountm.Increment(resolvedName)
		if isMounted(hostdir) {
			return &volume.MountResponse{Mountpoint: hostdir}, nil
		}
		log.Infof("Existing CIFS volume not mounted, force remount.")
		c.mountm.Decrement(resolvedName)
	}

	log.Infof("Mounting CIFS volume %s on %s", source, hostdir)

	if err := createDest(hostdir); err != nil {
		return nil, err
	}

	if err := c.mountVolume(resolvedName, source, hostdir, c.getCreds(host)); err != nil {
		os.Remove(hostdir)
		return nil, err
	}
	c.mountm.Add(resolvedName, hostdir)

	if c.mountm.GetOption(resolvedName, ShareOpt) != "" && c.mountm.GetOptionAsBool(resolvedName, CreateOpt) {
		log.Infof("Mount: Share and Create options enabled - using %s as sub-dir mount", resolvedName)
		datavol := filepath.Join(hostdir, resolvedName)
		if err := createDest(datavol); err != nil {
			return nil, err
		}
		hostdir = datavol
	}
	return &volume.MountResponse{Mountpoint: hostdir}, nil
}

// Unmount do the unmounting
func (c CifsDriver) Unmount(r *volume.UnmountRequest) error {
	c.m.Lock()
	defer c.m.Unlock()

	resolvedName, _ := resolveName(r.Name)
	hostdir := mountpoint(c.root, resolvedName)

	if c.mountm.HasMount(resolvedName) {
		if c.mountm.Count(resolvedName) > 1 {
			log.Infof("Skipping unmount for %s - in use by other containers", resolvedName)
			c.mountm.Decrement(resolvedName)
			return nil
		}
		c.mountm.Decrement(resolvedName)
	}

	log.Infof("Unmounting volume %s from %s", resolvedName, hostdir)

	if err := runUmount(hostdir); err != nil {
		return err
	}

	c.mountm.DeleteIfNotManaged(resolvedName)

	return nil
}

func (c CifsDriver) fixSource(name string) string {
	if c.mountm.HasOption(name, ShareOpt) {
		return "//" + c.mountm.GetOption(name, ShareOpt)
	}
	return "//" + name
}

func (c CifsDriver) parseHost(name string) string {
	n := name
	if c.mountm.HasOption(name, ShareOpt) {
		n = c.mountm.GetOption(name, ShareOpt)
	}

	if strings.ContainsAny(n, "/") {
		s := strings.Split(n, "/")
		return s[0]
	}
	return n
}

func (c CifsDriver) mountVolume(name, source, dest string, creds *CifsCreds) error {
	var opts []string
	var user = creds.user
	var pass = creds.pass
	var domain = creds.domain
	var security = creds.security
	var fileMode = creds.fileMode
	var dirMode = creds.dirMode

	options := merge(c.mountm.GetOptions(name), c.cifsopts)
	if val, ok := options[CifsOpts]; ok {
		opts = append(opts, val)
	}

	if c.mountm.HasOptions(name) {
		mopts := c.mountm.GetOptions(name)
		if v, found := mopts[UsernameOpt]; found {
			user = v
		}
		if v, found := mopts[PasswordOpt]; found {
			pass = v
		}
		if v, found := mopts[DomainOpt]; found {
			domain = v
		}
		if v, found := mopts[SecurityOpt]; found {
			security = v
		}
		if v, found := mopts[FileModeOpt]; found {
			fileMode = v
		}
		if v, found := mopts[DirModeOpt]; found {
			dirMode = v
		}
	}

	if user != "" {
		opts = append(opts, fmt.Sprintf("username=%s", user))
		if pass != "" {
			opts = append(opts, fmt.Sprintf("password=%s", pass))
		}
	} else {
		opts = append(opts, "guest")
	}

	if domain != "" {
		opts = append(opts, fmt.Sprintf("domain=%s", domain))
	}

	if security != "" {
		opts = append(opts, fmt.Sprintf("sec=%s", security))
	}

	if fileMode != "" {
		opts = append(opts, fmt.Sprintf("file_mode=%s", fileMode))
	}

	if dirMode != "" {
		opts = append(opts, fmt.Sprintf("dir_mode=%s", dirMode))
	}

	opts = append(opts, "rw")

	optStr := strings.Join(opts, ",")
	log.Debugf("Executing: mount -t cifs -o %s %s %s", strings.Replace(optStr, "password="+pass, "password=****", 1), source, dest)
	return runMount("cifs", optStr, source, dest)
}

func (c CifsDriver) getCreds(host string) *CifsCreds {
	log.Debugf("GetCreds: host=%s, netrc=%v", host, c.netrc)
	if c.netrc != nil {
		m := c.netrc.Machine(host)
		if m != nil {
			return &CifsCreds{
				user:     m.Get("username"),
				pass:     m.Get("password"),
				domain:   m.Get("domain"),
				security: m.Get("security"),
				fileMode: m.Get("fileMode"),
				dirMode:  m.Get("dirMode"),
			}
		}
	}
	return c.creds
}
