package dts

// package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"miracl/core/BLS12383"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	icore "github.com/ipfs/boxo/coreiface"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/interface-go-ipfs-core/path"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p/core/peer"
)

/// ------ Setting up the IPFS Repo

func setupPlugins(externalPluginsPath string) error {
	// Load any external plugins if available on externalPluginsPath
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

func createTempRepo() (string, error) {
	repoPath, err := os.MkdirTemp("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(io.Discard, 2048)
	if err != nil {
		return "", err
	}

	// When creating the repository, you can define custom settings on the repository, such as enabling experimental
	// features (See experimental-features.md) or customizing the gateway endpoint.
	// To do such things, you should modify the variable `cfg`. For example:
	if *flagExp {
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-filestore
		cfg.Experimental.FilestoreEnabled = true
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-urlstore
		cfg.Experimental.UrlstoreEnabled = true
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-p2p
		cfg.Experimental.Libp2pStreamMounting = true
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#p2p-http-proxy
		cfg.Experimental.P2pHttpProxy = true
		// See also: https://github.com/ipfs/kubo/blob/master/docs/config.md
		// And: https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md
	}

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

/// ------ Spawning the node

// Creates an IPFS node and returns its coreAPI
func createNode(ctx context.Context, repoPath string) (*core.IpfsNode, error) {
	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}
	cfg, err := repo.Config()

	fmt.Println("createNode config: ", cfg)

	// Sets swarm ports to random - helps with port conflicts
	maxTries := 3
	err = config.Profiles["randomports"].Transform(cfg)
	for i := 0; i < maxTries; i++ {
		if err == nil {
			break
		}
		err = config.Profiles["randomports"].Transform(cfg)
	}
	if err != nil {
		return nil, err
	}

	err = repo.SetConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Construct the node

	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption, // This option sets the node to be a full DHT node (both fetching and storing DHT Records)
		// Routing: libp2p.DHTClientOption, // This option sets the node to be a client DHT node (only fetching records)
		Repo: repo,
		// DisableEncryptedConnections: true,
		Permanent: true,
		// NilRepo:   true,
		ExtraOpts: map[string]bool{
			"pubsub": true,
		},
	}

	// fmt.Println("core.NewNode: ", nodeOptions)

	return core.NewNode(ctx, nodeOptions)
}

var loadPluginsOnce sync.Once

// Spawns a node to be used just for this run (i.e. creates a tmp repo)
func spawnEphemeral(ctx context.Context) (icore.CoreAPI, *core.IpfsNode, error) {
	var onceErr error
	loadPluginsOnce.Do(func() {
		onceErr = setupPlugins("")
	})
	if onceErr != nil {
		return nil, nil, onceErr
	}

	// Create a Temporary Repo
	repoPath, err := createTempRepo()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp repo: %s", err)
	}

	node, err := createNode(ctx, repoPath)
	if err != nil {
		return nil, nil, err
	}

	api, err := coreapi.NewCoreAPI(node)

	return api, node, err
}

func connectToPeers(ipfs icore.CoreAPI, peers []string) error {
	// var wg sync.WaitGroup
	peerInfos := make(map[peer.ID]*peer.AddrInfo, len(peers))
	for _, addrStr := range peers {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return err
		}
		pii, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return err
		}
		pi, ok := peerInfos[pii.ID]
		if !ok {
			pi = &peer.AddrInfo{ID: pii.ID}
			peerInfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}

	ctxA := context.Background()
	// wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peer.AddrInfo) {
			// defer wg.Done()
			err := ipfs.Swarm().Connect(ctxA, *peerInfo)
			if err != nil {
				log.Printf("failed to connect toooooooo %s", peerInfo.String())
				log.Printf("failed to connect to %s: %s", peerInfo.ID, err)
			}
		}(peerInfo)
	}
	// wg.Wait()
	return nil
}

// nodes.log.txtが無ければファイルを作成し，
// nodes.log.txtファイルが有れば作成せずに，
// 指定された文字列をファイルnodes.log.txtに追記する
func writeNodes(node string) {
	file, err := os.OpenFile("./log/nodes.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if _, err := file.WriteString(node + "\n"); err != nil {
		panic(err)
	}
}

// nodes.log.txtが無ければエラーを返し，nodes.log.txtファイルが有ればファイルの内容を行ごとに文字列の配列にして返す
func ReadNodes(mesh bool) []string {
	file, err := os.Open("./log/nodes.log")
	if !mesh {
		file, err = os.Open("localipfs")
	}
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var nodes []string
	for {
		var node string
		_, err := fmt.Fscanln(file, &node)
		if err != nil {
			break
		}
		nodes = append(nodes, node)
	}
	return nodes
}

/// -------

var flagExp = flag.Bool("experimental", false, "enable experimental features")

// var node *core.IpfsNode
var ctx context.Context

//var peerMa []string

func StartNode(mesh bool, test bool) (ipfs icore.CoreAPI, node *core.IpfsNode) {
	flag.Parse()
	/// --- Part I: Getting a IPFS node running
	fmt.Println("-- Getting an IPFS node running -- ")

	ctxA, _ := context.WithCancel(context.Background())
	// defer cancel()

	// Spawn a local peer using a temporary path, for testing purposes
	ipfsA, nodeA, err := spawnEphemeral(ctxA)
	if err != nil {
		panic(fmt.Errorf("failed to spawn peer node: %s", err))
	}

	// nodeIdentity := ipns.NameFromPeer(nodeA.Identity).String()
	nodeIdentity := ipns.NameFromPeer(nodeA.Identity).String()
	a, _ := ipfsA.Swarm().LocalAddrs(nodeA.Context())

	fmt.Println("node info : " + a[1].String())

	if !test {
		writeNodes(a[0].String() + "/p2p/" + nodeIdentity)
	}
	// 1秒のスリープ
	// time.Sleep(3 * time.Second)

	//ipfs = ipfsA
	//node = nodeA
	ctx = ctxA

	boot(nodeA, ipfsA, mesh)

	// addBin([]byte("test"))

	fmt.Println("IPFS node is running")
	return ipfsA, nodeA
}

func AddBin(data []byte, vn *VoteNode) (path.Resolved, error) {
	// fmt.Println(data)
	peerCidFile, err := vn.IPFS.Unixfs().Add(ctx, files.NewBytesFile(data))
	if err != nil {
		panic(fmt.Errorf("could not add File: %s", err))
	}

	fmt.Printf("Added file to peer with CID %s\n", peerCidFile.String())
	return peerCidFile, nil
}

// var peerMa []string

// boot sets up the IPFS network node for a VoteNode
func boot(node *core.IpfsNode, ipfs icore.CoreAPI, mesh bool) {
	// 3秒スリープ
	// time.Sleep(3 * time.Second)

	fmt.Println("-- Going to connect to a few nodes in the Network as bootstrappers --")

	// if peerMa == "" {
	// peerMa = append(peerMa, fmt.Sprintf("/ip4/127.0.0.1/udp/4010/p2p/%s", node.Identity.String()))
	// }

	nodeIdentity := ipns.NameFromPeer(node.Identity).String()
	// a, _ := ipfs.Swarm().LocalAddrs(node.Context())
	// fmt.Println("ipfs.Swarm().LocalAddresss", a)
	// b, _ := ipfs.Swarm().ListenAddrs(node.Context())
	// fmt.Println("ipfs.Swarm().ListenAddrs", b)
	// peerMa := a[0].String() + "/p2p/" + nodeIdentity
	// peerMa := fmt.Sprintf("/ip4/127.0.0.1/udp/4010/p2p/%s", ipns.NameFromPeer(node.Identity))
	// fmt.Println("peerMa: ", peerMa)

	// bootstrapNodes := []string{
	// 	// IPFS Bootstrapper nodes.
	// 	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	// 	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	// 	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	// 	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",

	// 	// // IPFS Cluster Pinning nodes
	// 	// "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",

	// 	// You can add more nodes here, for example, another IPFS node you might have running locally, mine was:
	// 	// "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWQ4sGyJr8mFxqsto1fedcAPvwXDcMGH3zsRFpWCapePyb",
	// 	// "/ip4/127.0.0.1/udp/4001/quic-v1/p2p/12D3KooWQ4sGyJr8mFxqsto1fedcAPvwXDcMGH3zsRFpWCapePyb",
	// 	// "/ip4/127.0.0.1/udp/4001/quic-v1/webtransport/certhash/uEiC9VCX05obpgjaCZ5cq61uG2COMCdv_uw7yXvl2pkTbEw/certhash/uEiA-5ugH_jcGiRp0FAt4UwqNPTNYJFLICobZs3pt-NZwXg/p2p/12D3KooWQ4sGyJr8mFxqsto1fedcAPvwXDcMGH3zsRFpWCapePyb",
	// 	// "/ip4/127.0.0.1/udp/4001/quic/p2p/12D3KooWQ4sGyJr8mFxqsto1fedcAPvwXDcMGH3zsRFpWCapePyb",
	// 	// "/ip4/173.82.120.112/tcp/4001/p2p/12D3KooWGEaBMjzc9tjkNPzbPNjg7cgHpX52TcJdj9E8s1DA36iV/p2p-circuit/p2p/12D3KooWGEtQggFuw1nUm9KYrk7Nf5B3i1jGGofLnAoC6fmMfvXR",
	// }

	// bootstrapNodes = append(bootstrapNodes, peerMa)

	bootstrapNodes := ReadNodes(mesh)

	fmt.Println("readNodes: ", bootstrapNodes)
	// fmt.Println("peerMa: ", peerMa)
	// bootstrapNodesからpeerMaと一致する要素を削除
	for i, v := range bootstrapNodes {
		if strings.Contains(v, nodeIdentity) {
			// fmt.Println("readNodes: ", strconv.Itoa(i))
			bootstrapNodes = append(bootstrapNodes[:i], bootstrapNodes[i+1:]...)
		}
	}

	// fmt.Println("removed nodes: ", bootstrapNodes)

	err := connectToPeers(ipfs, bootstrapNodes)
	if err != nil {
		log.Printf("failed connect to peers: %s", err)
	}

	//	go func() {
	//		//err := connectToPeers(ctx, ipfs, bootstrapNodes)
	//		err := connectToPeers(ctx, ipfs, bootstrapNodes)
	//		if err != nil {
	//			log.Printf("failed connect to peers: %s", err)
	//		}
	//	}()
}

// pubsubs with peers to exchange targets
func (vn *VoteNode) pubsubTargets(tu int64, beginningtime time.Time) []Target {
	// create unixtime variable
	unixtime := strconv.FormatInt(tu, 10)
	//logging(vn, "pubsub target unixtime: "+unixtime)
	checktime := strconv.FormatInt(time.Now().Unix()/Joininterval, 10)
	//logging(vn, "pubsub target checktime: "+checktime)

	//allows problem nodes to join properly, but does not prevent skipping timestamps
	if unixtime != checktime {
		logging(vn, "checktime and unixtime mismatch")
	}

	//subscribe to topic (unixtime)
	currentpubsub, err := vn.IPFS.PubSub().Subscribe(Loopctx, unixtime)
	if err != nil {
		panic(fmt.Errorf("subscribe error: %s", err))
	}
	logging(vn, "subscribe at:  "+time.Since(beginningtime).String())
	logging(vn, "subscribe at: "+time.Now().String())

	//publish to topic in seperate thread
	go func(unixtime string, vn *VoteNode) {
		time.Sleep(time.Second * time.Duration(Joininterval*(4/3)))

		if vn.FirstPublish {
			vn.FirstPublish = false
		} else {
			// vn.updateFile()
		}
		tar := vn.CreateTarget()
		mes, _ := MarshalTarget(tar)

		err = vn.IPFS.PubSub().Publish(Loopctx, unixtime, mes)
		// // logging(vn, "publish time: "+time.Since(beginningtime).String())
		// // logging(vn, "publish time: "+time.Now().String())
		if err != nil {
			panic(fmt.Errorf("publish error: %s", err))
		}
		logging(vn, "publish time: "+time.Since(beginningtime).String())
		logging(vn, "publish time: "+time.Now().String())
	}(unixtime, vn)

	var targets []Target

	timeoutctx, cancel := context.WithTimeout(Loopctx, time.Second*((8/3)*time.Duration(Joininterval)))
	defer cancel()

	// logging(vn, "starting loop")
	logging(vn, "start recieve at: "+time.Since(beginningtime).String())
	logging(vn, "start recieve at: "+time.Now().String())

	iterations := 0
	for start := time.Now(); time.Since(start) < time.Second*100; {
		// logging(vn, "beginning of loop")
		// // logging(vn, "time: "+time.Since(beginningtime).String())
		// // logging(vn, "time: "+time.Now().String())
		PubSubMessage, error := currentpubsub.Next(timeoutctx)
		if error != nil {
			fmt.Println(error)
			logging(vn, "time of context break: "+time.Since(beginningtime).String())
			logging(vn, "time of context break: "+time.Now().String())
			// logging(vn, "end receive at: "+strconv.FormatInt(time.Now().Unix(), 10)+", "+strconv.FormatInt((time.Now().Unix()/joininterval), 10))
			break
		}
		iterations++
		// // logging(vn, "message from: "+PubSubMessage.From().String())
		// // logging(vn, "time after: "+time.Since(beginningtime).String())
		// // logging(vn, "time after: "+time.Now().String())

		rectar, err := UnmarshalTarget(PubSubMessage.Data())
		if err != nil {
			fmt.Println(err)
		}
		// logging(vn, "target message length: "+strconv.Itoa(len(rectar.Hash)))
		targets = append(targets, rectar)
		// logging(vn, "end of loop")
	}
	// logging(vn, "finished pubsubTargets")
	// w.Flush()
	err = currentpubsub.Close()

	return targets
}

// pubsubs with peers to exchange signaturedata
func (vn *VoteNode) pubsubSigs(tu int64, beginningtime time.Time, sigma SignatureData) (int, []SignatureData) {

	unixtime := strconv.FormatInt(tu, 10)
	// logging(vn, "pubsub sigs unixtime: "+unixtime)
	checktime := strconv.FormatInt(time.Now().Unix()/Joininterval, 10)
	// // logging(vn, "pubsub sigs checktime: "+checktime)

	//allows problem nodes to join properly, but does not prevent skipping timestamps
	if unixtime != checktime {
		logging(vn, "checktime and unixtime mismatch")
	}

	secondpubsub, err := vn.IPFS.PubSub().Subscribe(Loopctx, unixtime+"signed")
	if err != nil {
		panic(fmt.Errorf("subscribe error: %s", err))
	}
	// // logging(vn, "subscribe at:  "+time.Since(beginningtime).String())
	// // logging(vn, "subscribe at: "+time.Now().String())

	//publish to topic in seperate thread
	go func(unixtime string, vn *VoteNode) {
		time.Sleep(time.Second * time.Duration(Joininterval*(4/3)))
		sigmajson, err := MarshalSignature(sigma)
		err = vn.IPFS.PubSub().Publish(Loopctx, unixtime+"signed", sigmajson)
		// // logging(vn, "publish time: "+time.Since(beginningtime).String())
		// // logging(vn, "publish time: "+time.Now().String())
		if err != nil {
			panic(fmt.Errorf("publish error: %s", err))
		}
		// // logging(vn, "publish time: "+time.Since(beginningtime).String())
		// // logging(vn, "publish time: "+time.Now().String())
	}(unixtime, vn)

	timeoutctx, cancel := context.WithTimeout(Loopctx, time.Second*((8/3)*time.Duration(Joininterval)))
	defer cancel()

	// // logging(vn, "starting loop")
	// // logging(vn, "start recieve at: "+time.Since(beginningtime).String())
	// // logging(vn, "start recieve at: "+time.Now().String())

	var recievedSignatures []SignatureData
	iterations := 0
	for start := time.Now(); time.Since(start) < time.Second*100; {
		// // logging(vn, "beginning of loop")
		// // logging(vn, "time: "+time.Since(beginningtime).String())
		// // logging(vn, "time: "+time.Now().String())
		PubSubMessage, error := secondpubsub.Next(timeoutctx)
		if error != nil {
			fmt.Println(error)
			// logging(vn, "time of context break: "+time.Since(beginningtime).String())
			// logging(vn, "time of context break: "+time.Now().String())
			// logging(vn, "end receive at: "+strconv.FormatInt(time.Now().Unix(), 10)+", "+strconv.FormatInt((time.Now().Unix()/joininterval), 10))
			break
		}
		iterations++
		// // logging(vn, "message from : "+PubSubMessage.From().String())
		// // logging(vn, "time after: "+time.Since(beginningtime).String())
		// // logging(vn, "time after: "+time.Now().String())

		recsig, err := UnmarshalSignature(PubSubMessage.Data())
		if err != nil {
			fmt.Println(err)
		}
		recievedSignatures = append(recievedSignatures, recsig)
		// // logging(vn, "end of loop")
	}
	// // logging(vn, "finished loop")
	secondpubsub.Close()

	// w.Flush()

	return iterations, recievedSignatures
}

// used in MassPublish to publish a massive amount of targets, without regard for retrieving them
func (vn *VoteNode) pubTargets(tu int64, beginningtime time.Time) ([]Target, []*BLS12383.BIG) {
	// beginningtime := time.Now()

	// create unixtime variable
	unixtime := strconv.FormatInt(tu, 10)
	//logging(vn, "pubsub target unixtime: "+unixtime)
	checktime := strconv.FormatInt(time.Now().Unix()/Joininterval, 10)

	//allows problem nodes to join properly, but does not prevent skipping timestamps
	if unixtime != checktime {
		logging(vn, "checktime and unixtime mismatch")
	}

	var rettar []Target
	var skeys []*BLS12383.BIG

	time.Sleep(time.Second * time.Duration(Joininterval*(4/3)))
	//publish to topic in seperate thread
	for i := 0; i < 1000; i++ {
		if vn.FirstPublish {
			vn.FirstPublish = false
		}
		tar, skey := vn.GenericTarget()
		rettar = append(rettar, tar)
		skeys = append(skeys, skey)
		mes, _ := MarshalTarget(tar)

		err := vn.IPFS.PubSub().Publish(Loopctx, unixtime, mes)
		if err != nil {
			panic(fmt.Errorf("publish error: %s", err))
		}
	}

	return rettar, skeys
}

// used in MassPublish to publish a massive amount of signatures, without regard for retrieving them
func (vn *VoteNode) pubSigs(tu int64, beginningtime time.Time, sigmas []SignatureData) {
	// beginningtime := time.Now()

	unixtime := strconv.FormatInt(tu, 10)
	// logging(vn, "pubsub sigs unixtime: "+unixtime)
	checktime := strconv.FormatInt(time.Now().Unix()/Joininterval, 10)
	// logging(vn, "pubsub sigs checktime: "+checktime)

	//allows problem nodes to join properly, but does not prevent skipping timestamps
	if unixtime != checktime {
		logging(vn, "checktime and unixtime mismatch")
	}

	time.Sleep(time.Second * time.Duration(Joininterval*(4/3)))
	//publish to topic in seperate thread
	for i := 0; i < len(sigmas); i++ {

		sigmajson, err := MarshalSignature(sigmas[i])
		err = vn.IPFS.PubSub().Publish(Loopctx, unixtime+"signed", sigmajson)
		// logging("publish this: "+encryptedID.String())
		// logging(vn, "publish time: "+time.Since(beginningtime).String())
		// logging(vn, "publish time: "+time.Now().String())
		if err != nil {
			panic(fmt.Errorf("publish error: %s", err))
		}
		// logging(vn, "publish time: "+time.Since(beginningtime).String())
		// logging(vn, "publish time: "+time.Now().String())
	}
}
