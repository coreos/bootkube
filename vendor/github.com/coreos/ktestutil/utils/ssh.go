package utils

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	// defaultSSHPort is the default port used for ssh.
	defaultSSHPort = 22
	// defaultSSHUser is the default user user for ssh.
	defaultSSHUser = "core"
	// defaultSSHTimeout is the default maximum amount of time for the ssh connection to establish.
	defaultSSHTimeout = 30 * time.Second
)

type SSHConfig struct {
	// default to `core`
	User string
	// defaults to `22`
	Port int32
	// uses `SSH_AUTH_SOCK` if this is empty
	IdentifyKeyFile string
	// Timeout is the maximum amount of time for the ssh connection to establish.
	// defaults to `30 secs`
	Timeout time.Duration
}

// SSHClient holds config for ssh.
type SSHClient struct {
	*ssh.ClientConfig
	port int32
}

// MustNewSSHClient returns *SSHClient.
// Uses default values for SSHConfig.
func MustNewSSHClient(config *SSHConfig) *SSHClient {
	var authMethod ssh.AuthMethod
	sock := os.Getenv("SSH_AUTH_SOCK")
	switch {
	case config.IdentifyKeyFile != "":
		key, err := ioutil.ReadFile(config.IdentifyKeyFile)
		if err != nil {
			glog.Fatalf("error reading key file: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			glog.Fatalf("error parsing private key: %v", err)
		}
		authMethod = ssh.PublicKeys(signer)
		break

	case sock != "":
		sshAgent, err := net.Dial("unix", sock)
		if err != nil {
			glog.Fatalf("error connecting to ssh-agent: %v", err)
		}
		authMethod = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
		break

	default:
		glog.Fatalf("no ssh connection authentication provided")
	}

	if config.User == "" {
		config.User = defaultSSHUser
	}
	if config.Port == 0 {
		config.Port = defaultSSHPort
	}
	if config.Timeout == 0 {
		config.Timeout = defaultSSHTimeout
	}
	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         config.Timeout,
	}
	sshConfig.SetDefaults()

	return &SSHClient{
		port:         config.Port,
		ClientConfig: sshConfig,
	}
}

// Exec executes the cmd on given host.
func (c *SSHClient) Exec(host, cmd string) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return c.ExecWithCtx(ctx, host, cmd)
}

// ExecWithCtx executes cmd on given host, with ctx.
func (c *SSHClient) ExecWithCtx(ctx context.Context, host, cmd string) (stdout, stderr []byte, err error) {
	if host == "" {
		return nil, nil, fmt.Errorf("error: empty host provided")
	}

	endpoint := fmt.Sprintf("%s:%d", host, c.port)
	client, err := ssh.Dial("tcp", endpoint, c.ClientConfig)
	if err != nil {
		return nil, nil, err
	}
	defer client.Conn.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, err
	}
	defer session.Close()

	outBuf := bytes.NewBuffer(nil)
	errBuf := bytes.NewBuffer(nil)
	session.Stdout = outBuf
	session.Stderr = errBuf

	runCh := make(chan struct{})
	go func() {
		err = session.Run(cmd)
		runCh <- struct{}{}
	}()

	select {
	case <-runCh:
		stdout = outBuf.Bytes()
		stderr = errBuf.Bytes()
		return stdout, stderr, err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// NewScpClient returns scp client with given host and ssh client.
func NewScpClient(sshClient *SSHClient, host string) (*sftp.Client, error) {
	if host == "" {
		return nil, fmt.Errorf("error: empty host provided")
	}

	endpoint := fmt.Sprintf("%s:%d", host, sshClient.port)
	client, err := ssh.Dial("tcp", endpoint, sshClient.ClientConfig)
	if err != nil {
		return nil, err
	}

	return sftp.NewClient(client)
}
