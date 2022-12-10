package utils

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/armon/go-socks5"
	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	PrivateKey         []byte
	KnownHostsFilepath string
	SSHHost            string
	SSHPort            string
	SSHUsername        string
	SocksV5IP          string
	SocksV5Port        string
}

func (config *SSHConfig) StartSocksV5Server() {
	// References:
	// 1. https://gist.github.com/afdalwahyu/4c70868c84e68676c86e1a54b410655d
	// 2. https://pkg.go.dev/golang.org/x/crypto/ssh#PublicKeys
	// 3. https://stackoverflow.com/questions/45441735/ssh-handshake-complains-about-missing-host-key
	sshConn, err := config.connectToSSH()
	if err != nil {
		log.Fatalf("SSH Connection failed with error: %s", err)
	}
	defer func(sshConn *ssh.Client) {
		err := sshConn.Close()
		if err != nil {
			log.Warnf("Error occurred when trying to close the SSH Connection. Error: %s\n", err)
		}
	}(sshConn)
	log.Infoln("Connected to ssh server")
	go func() {
		conf := &socks5.Config{
			Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return sshConn.Dial(network, addr)
			},
		}
		serverSocks, err := socks5.New(conf)
		if err != nil {
			fmt.Println(err)
			return
		}
		socksV5Address := fmt.Sprintf("%s:%s", config.SocksV5IP, config.SocksV5Port)
		if err := serverSocks.ListenAndServe("tcp", socksV5Address); err != nil {
			log.Fatalf("Failed to create socks5 server %s", err)
		}
		exitCh := make(chan os.Signal)
		signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-exitCh
			cleanup()
			os.Exit(0)
		}()
	}()
	log.Infoln("Started SocksV5 server.")
	log.Infoln("Press CTRL+C to stop SocksV5 server and exit!")
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	return
}

func cleanup() {
	log.Infoln("Stopping SocksV5 Server")
	log.Infoln("Disconnecting SSH Connection.")
	log.Infoln("Terminating EC2 Instance.")
	log.Infoln("All clean up done without any errors.")
	log.Infoln("Exiting...")
}

func (config *SSHConfig) connectToSSH() (*ssh.Client, error) {
	signer, err := ssh.ParsePrivateKey(config.PrivateKey)
	if err != nil {
		log.Fatalf("Unable to parse private key: %v", err)
	}
	sshConf := &ssh.ClientConfig{
		User:            config.SSHUsername,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	hostWithPort := fmt.Sprintf("%s:%s", config.SSHHost, config.SSHPort)
	sshConn, err := ssh.Dial("tcp", hostWithPort, sshConf)
	return sshConn, err
}