package dts

// package main

import (
	"context"
	"miracl/core/BLS12383"
	"os"

	icore "github.com/ipfs/boxo/coreiface"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core"
)

var Loopctx context.Context
var Joininterval int64 = 3

var P = 1.0
var mesh = false

var name = 0

type SignatureData struct {
	SignedHash []byte //*BLS12383.ECP
	Target     Target
}

type Target struct {
	Name   string
	Hash   []byte
	PubKey []byte //*BLS12383.ECP2
}

type FileHistory struct {
	OwnerID        string //IPNS
	Time           int64
	Message        []byte
	CurrentVersion int
	PreviousCID    []byte
	AggregateBytes []byte
}

// parameters here are stored as bytes to help wiht Unmarshaling
// contains a list of keys, names, the latest FileHistory, and a list of CIDData representing all FileHistory in the chain
type ChainHead struct {
	Keys        [][]byte
	Names       []string
	Filehistory []byte //FileHistory
	CIDList     []byte //[]CIDList
}

// combines a CID of a FileHistory, and the unixtime in which the FileHistory was created
type CIDData struct {
	CID  []byte
	time int64
}

// combines a name and unixtime
type NameTime struct {
	Name string
	time int64
}

// contains a subchain of a chain dictated by the name, and the list of names from peers who participated
type SubChain struct {
	subchain []CIDData
	names    []string
	name     string
}

// type AggregateData struct {
// 	sigmaagg *BLS12383.ECP
// 	messages [][]byte
// 	pubkeys  []*BLS12383.ECP2
// 	table    [][]bool
// }

type AggregateBytes struct {
	Sigmaagg []byte
	Messages []byte //change to string(?)
	//Hashes  []byte
	Indexes []int
	Table   []byte
	Rows    int
}

type VoteNode struct {
	ID           string
	IPFS         icore.CoreAPI
	Node         *core.IpfsNode
	CurrentFile  FileHistory
	CurrentCID   cid.Cid
	Name         ipns.Name
	FirstPublish bool
	Key1         *BLS12383.ECP
	Key2         *BLS12383.ECP2
	KeyS         *BLS12383.BIG
	KeyList      [][]byte
	NameList     []string
	msgTmp       []byte //change to string
	LogFile      *os.File
	ChainAddress string
	CIDList      []CIDData
}

type EthereumBlock struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      int          `json:"id"`
	Result  *BlockResult `json:"result"`
}

type BlockResult struct {
	Number           string   `json:"number"`
	Hash             string   `json:"hash"`
	MixHash          string   `json:"mixHash"`
	ParentHash       string   `json:"parentHash"`
	Nonce            string   `json:"nonce"`
	Sha3Uncles       string   `json:"sha3Uncles"`
	LogsBloom        string   `json:"logsBloom"`
	TransactionsRoot string   `json:"transactionsRoot"`
	StateRoot        string   `json:"stateRoot"`
	ReceiptsRoot     string   `json:"receiptsRoot"`
	Miner            string   `json:"miner"`
	Difficulty       string   `json:"difficulty"`
	TotalDifficulty  string   `json:"totalDifficulty"`
	ExtraData        string   `json:"extraData"`
	Size             string   `json:"size"`
	GasLimit         string   `json:"gasLimit"`
	GasUsed          string   `json:"gasUsed"`
	Timestamp        string   `json:"timestamp"`
	Uncles           []string `json:"uncles"`
	Transactions     []string `json:"transactions"`
}
