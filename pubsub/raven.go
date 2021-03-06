package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	_ "time"
        "strconv"
	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
        host "github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-net"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
        libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-crypto"
	gossipsub "github.com/rairyx/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
)

var logger = log.Logger("raven")
var ho host.Host

var TopicName string = "RDEpsjSPrAZF9JCK5REt3tao"



func init() {
	         os.Setenv("GODEBUG", os.Getenv("GODEBUG")+",tls13=1")
}

func parseArgs() (bool, string, int) {
	usage := fmt.Sprintf("Usage: %s PRIVATE_KEY  PORT [--bootstrapper] \n\nPRIVATE_KEY is the path to a private key like '../util/private_key.bin'\n PORT is port to listen on, default is 6000\n--bootstrapper to run in bootstrap mode (creates a DHT and listens for peers)\n", os.Args[0])
	var bBootstrap bool = false
	var privKeyFilePath string
	var listenPort = 6000
	var args []string = os.Args[1:]
	if (len(args) == 0) || (len(args) > 2) {
		fmt.Printf("Error: wrong number of arguments\n\n%s", usage)
		os.Exit(1)
	}
	privKeyFilePath = args[0]
	if (len(args) == 2) && (args[1] == "--bootstrapper") {
		bBootstrap = true
	}else if (len(args) == 2) {
		listenPort, _ = strconv.Atoi(args[1])
	}
	return bBootstrap, privKeyFilePath, listenPort
}

func main() {
	ctx := context.Background()

	bBootstrap, privKeyFilePath, port := parseArgs()
	fmt.Printf("Starting up in ")
	if bBootstrap {
		fmt.Printf("bootstrapper mode (port 5555)")
	} else {
		fmt.Printf("peer mode : %d", port)
	}
	fmt.Printf("\nPrivate key '%s'\n", privKeyFilePath)

	//
	// Read the private key and unmarshall it into struct
	//
	var privBytes []byte
	privBytes, err := ioutil.ReadFile(privKeyFilePath)
	if err != nil {
		fmt.Println("ioutil.ReadFile:  failed:  %v", err)
		panic(err)
	}

	var priv crypto.PrivKey
	priv, err = crypto.UnmarshalPrivateKey(privBytes)
	if err != nil {
		fmt.Println("crypto.UnmarshalPrivateKey:  failed:  %v", err)
		panic(err)
	}

	//
	// Construct our libp2p host
	//
	var host host.Host
	if bBootstrap {
		host, err = libp2p.New(ctx,
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/5555"),
			libp2p.Identity(priv),
		)
	} else {
		host, err = libp2p.New(ctx,
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/6000"),
			libp2p.Identity(priv),
		)
	}
	if err != nil {
		fmt.Println("libp2p.New:  failed:  %v", err)
		panic(err)
	}

	logger.Info("Host created. We are:", host.ID())
	kademliaDHT, err := libp2pdht.New(ctx, host)
	if err != nil {
		panic(err)
	}
	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	logger.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
			panic(err)
	}
	// Construct a gossipsub instance for this host
	gsub, err := gossipsub.NewGossipSub(ctx, host, gossipsub.WithMessageSigning(false))
	if err != nil {
		fmt.Println("Error (gossipsub.NewGossipSub): %v", err)
		panic(err)
	}

	//
	// If we are the bootstrap node, we don't try to connect to any peers.
	// Else:  try to connect to the bootstrap node.
	//
	const bootstrapAddrIP4Str string = "127.0.0.1"
	if !bBootstrap {
		var bootstrapMultiAddr ma.Multiaddr
		var pinfo *peerstore.PeerInfo
		bootstrapMultiAddrStr := fmt.Sprintf("/ip4/%s/tcp/5555/ipfs/QmehVYruznbyDZuHBV4vEHESpDevMoAovET6aJ9oRuEzWa", bootstrapAddrIP4Str)
		fmt.Printf("bootstrapping to '%s'...\n", bootstrapMultiAddrStr)
		bootstrapMultiAddr, err := ma.NewMultiaddr(bootstrapMultiAddrStr)
		if err != nil {
			fmt.Println("Error (ma.NewMultiaddr): %v", err)
			panic(err)
		}

		pinfo, err = peerstore.InfoFromP2pAddr(bootstrapMultiAddr)
		if err != nil {
			fmt.Println("Error (ma.NewMultiaddr): %v", err)
			panic(err)
		}

		if err := host.Connect(ctx, *pinfo); err != nil {
			fmt.Println("bootstrapping to peer failed: ", err)
		}
	}

	//
	// Subscribe to the topic and wait for messages published on that topic
	//
	sub, err := gsub.Subscribe(TopicName)
	if err != nil {
		fmt.Println("Error (fsub.Subscribe): %v", err)
		panic(err)
	}

	// Go and listen for messages from them, and print them to the screen
	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				fmt.Println("Error (sub.Next): %v", err)
				panic(err)
			}

			//fmt.Printf("%s: %s\n", msg.GetFrom(), string(msg.GetData()))
			fmt.Printf("%s\n", string(msg.GetData()))
		}
	}()

	// SetConnHandler() should not normally be called.  Instead,
	// use Notify() and pass it a functioon. (The problem with
	// SetConnHandler() is that it takes control of the connection.)
	host.Network().Notify(&inet.NotifyBundle{
		ConnectedF: func(n inet.Network, c inet.Conn) {
			fmt.Println("Got a connection:", c.RemotePeer())
		},
	})
	if bBootstrap {
		fmt.Println("Bootstrapper running.\nPubSub object instantiated using GossipSubRouter.\nCtrl+C to exit.")
		select {}
	} else {
		// Now, wait for input from the user, and send that out!
		scan := bufio.NewScanner(os.Stdin)
		for scan.Scan() {
			if err := gsub.Publish(TopicName, scan.Bytes()); err != nil {
				panic(err)
			}
		}
	}
}
