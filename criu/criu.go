package criu

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containerd/containerd/reaper"
	"github.com/gogo/protobuf/proto"

	"golang.org/x/sys/unix"
)

const descriptiors = "descriptors.json"

type Opt func(*Client)

func New(path string, opts ...Opt) (*Client, error) {
	c := &Client{
		Path: path,
	}
	for _, o := range opts {
		o(c)
	}
	if err := os.MkdirAll(c.WorkDir, 0755); err != nil {
		return nil, err
	}
	f, err := os.Open(c.WorkDir)
	if err != nil {
		os.RemoveAll(c.WorkDir)
		return nil, err
	}
	c.work = f
	return c, nil
}

type Client struct {
	Path    string
	WorkDir string

	work *os.File
}

func (c *Client) Checkpoint(pid int, root, imageDir string, opts ...CheckpointOpt) error {
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return err
	}
	image, err := os.Open(imageDir)
	if err != nil {
		return err
	}
	defer image.Close()
	fds, err := c.collectFDs(pid)
	if err != nil {
		return err
	}
	options := c.baseOpts(pid, root, image)
	for _, o := range opts {
		if err := o(options); err != nil {
			return err
		}
	}
	f, err := os.Create(filepath.Join(imageDir, descriptiors))
	if err != nil {
		return err
	}
	err = json.NewEncoer(f).Encode(fds)
	f.Close()
	if err != nil {
		return err
	}
	swrk, err := c.newServer()
	if err != nil {
		return err
	}
	defer swrk.Close()
	if err := swrk.Start(); err != nil {
		return err
	}
	t := CriuReqType_DUMP
	if err := swrk.Send(&CriuReq{
		Type: &t,
		Opts: options,
	}); err != nil {
		return err
	}
	for {
		response, err := swrk.Receive()
		if err != nil {
			return err
		}
		switch response.GetType() {
		case CriuReqType_NOTIFY:
			if err := c.processNotification(response); err != nil {
				return err
			}
			t = CriuReqType_NOTIFY
			if err := swrk.Send(&CriuReq{
				Type:          &t,
				NotifySuccess: proto.Bool(true),
			}); err != nil {
				return err
			}
			continue
		case CriuReqType_RESTORE:
			break
		case CriuReqType_DUMP:
			break
		case CriuReqType_PRE_DUMP:
			// In pre-dump mode CRIU is in a loop and waits for
			// the final DUMP command.
			// The current runc pre-dump approach, however, is
			// start criu in PRE_DUMP once for a single pre-dump
			// and not the whole series of pre-dump, pre-dump, ...m, dump
			// If we got the message CriuReqType_PRE_DUMP it means
			// CRIU was successful and we need to forcefully stop CRIU
			swrk.Close()
			reaper.Default.Wait(swrk.cmd)
			return nil
		default:
			return fmt.Errorf("criu: unable to parse response type %s", response)
		}
	}
	if _, err := reaper.Default.Wait(server.cmd); err != nil {
		return err
	}
	return nil
}

func (c *Client) Restore() (int, error) {

}

func (c *Client) Close() (err error) {
	err = c.work.Close()
	if rerr := os.RemoveAll(c.WorkDir); err == nil {
		err = rerr
	}
	return err
}

func (c *Client) processNotification(response *CriuResp) error {
	notify := response.GetNotify()
	if notify == nil {
		return fmt.Errorf("invalid NOTIFY response: %s", response)
	}
	switch notify.GetScript() {
	case "post-dump":
	case "network-unlock":
	case "network-lock":
	case "setup-namespaces":
	case "post-restore":
	}
}

func (c *Client) baseOpts(pid int, root string, image *os.File) *CriuOpts {
	return CriuOpts{
		ImagesDirFd:    proto.Int32(int32(image.Fd())),
		WorkDirFd:      proto.Int32(int32(c.work.Fd())),
		LogLevel:       proto.Int32(4),
		LogFile:        proto.String("dump.log"),
		Root:           proto.String(root),
		Pid:            proto.Int32(int32(pid)),
		ManageCgroups:  proto.Bool(true),
		NotifyScripts:  proto.Bool(true),
		ShellJob:       proto.Bool(criuOpts.ShellJob),
		LeaveRunning:   proto.Bool(criuOpts.LeaveRunning),
		TcpEstablished: proto.Bool(criuOpts.TcpEstablished),
		ExtUnixSk:      proto.Bool(criuOpts.ExternalUnixConnections),
		FileLocks:      proto.Bool(criuOpts.FileLocks),
		EmptyNs:        proto.Uint32(criuOpts.EmptyNs),
	}
}

func (c *Client) newServer() (*server, error) {
	fds, err := unix.Socketpair(syscall.AF_LOCAL, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(c.Path, "swrk", "3")
	cmd.ExtraFiles = append(cmd.ExtraFiles, os.NewFile(uintptr(fds[1]), "criu-server"))
	return &server{
		fds:    fds,
		cmd:    cmd,
		client: os.NewFile(uintptr(fds[0], "criu-client")),
	}, nil
}

func (c *Client) collectFDs(pid int) ([]string, error) {
	var (
		fds     []string
		dirPath = filepath.Join("/proc", strconv.Itoa(pid), "/fd")
	)
	for i := 0; i < 3; i++ {
		// XXX: This breaks if the path is not a valid symlink (which can
		//      happen in certain particularly unlucky mount namespace setups).
		f := filepath.Join(dirPath, strconv.Itoa(i))
		target, err := os.Readlink(f)
		if err != nil {
			// Ignore permission errors, for rootless containers and other
			// non-dumpable processes. if we can't get the fd for a particular
			// file, there's not much we can do.
			if os.IsPermission(err) {
				continue
			}
			return fds, err
		}
		fds[i] = target
	}
	return fds, nil
}

type server struct {
	fds    [2]int
	cmd    *exec.Cmd
	client *os.File
}

func (s *server) Start() error {
	if err := reaper.Default.Start(cmd); err != nil {
		return err
	}
	return unix.Close(s.serverFD())
}

func (s *server) Send(opts *CriuReq) error {
	data, err := proto.Marshal(opts)
	if err != nil {
		return err
	}
	_, err = swrk.Write(data)
	return err
}

func (s *server) Receive() (*CriuResp, error) {
	buf := make([]byte, 10*4096)
	n, err := s.Read(buf)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("unexpected EOF")
	}
	if n == len(buf) {
		return nil, fmt.Errorf("buffer is too small")
	}
	var response CriuResp
	if err := proto.Unmarshal(buf[:n], &response); err != nil {
		return err
	}
	if !response.GetSuccess() {
		return nil, fmt.Errorf("criu: %s error %d", response.GetType(), response.GetCrErrno())
	}
	return &response, nil
}

func (s *server) Write(b []byte) (int, error) {
	return s.client.Write(b)
}

func (s *server) Read(b []byte) (int, error) {
	return s.client.Read(b)
}

func (s *server) serverFD() int {
	return s.fds[1]
}

func (s *server) clientFD() int {
	return s.fds[0]
}

func (s *server) Close() (err error) {
	// no need to wait here as we use a global subreaper and we will make
	// sure the criu binary is properly reaped without a call to Wait()
	for _, fd := range s.fds {
		if cerr := unix.Close(fd); err == nil {
			err = cerr
		}
	}
	return err
}
