package main

import (
	"context"
	"io"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
)

const (
	Peers   = "peerList"
	PortNbr = "portNbr"
)

func init() {
	f, err := os.OpenFile("masa_node_lite.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		logrus.Fatal(err)
	}
	mw := io.MultiWriter(os.Stdout, f)
	logrus.SetOutput(mw)
	logrus.SetLevel(logrus.DebugLevel)
}

func main() {
	logrus.Infof("arg size is %d", len(os.Args))
	if len(os.Args) > 1 {
		logrus.Infof("found arg: %s", os.Args[1])
		err := os.Setenv(Peers, os.Args[1])
		if err != nil {
			logrus.Error(err)
		}
		if len(os.Args) == 3 {
			err := os.Setenv(PortNbr, os.Args[2])
			if err != nil {
				logrus.Error(err)
			}
		}
	}
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Listen for SIGINT (CTRL+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Cancel the context when SIGINT is received
	go func() {
		<-c
		cancel()
	}()

	privKey, err := CreatePrivateKey()
	if err != nil {
		logrus.Fatal(err)
	}
	node, err := NewNodeLite(privKey, ctx)
	if err != nil {
		logrus.Fatal(err)
	}

	node.Start()
	<-ctx.Done()
}
