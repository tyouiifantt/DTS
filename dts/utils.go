package dts

// package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"

	// miracl "miracl/core"
	"miracl/core/BLS12383"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	icore "github.com/ipfs/boxo/coreiface"
	"github.com/ipfs/boxo/coreiface/options"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

func logging(vn *VoteNode, message string) {
	logw := bufio.NewWriter(vn.LogFile)
	_, err := fmt.Fprint(logw, message+"\n")
	if err != nil {
		panic(err)
	}
	logw.Flush()
}

// Converts Target to a String
func (t Target) String() string {
	return fmt.Sprintf("%s : %s", t.Hash, t.PubKey)
}

// takes 2 byte slices, and returns a XOR byte slice of the two slices
func byteXOR(a []byte, b []byte) []byte {
	if len(a) != len(b) {
		if len(a) > len(b) {
			temp := a
			a = b
			b = temp
		}
	}
	temphash := make([]byte, len(b))
	for i := range b {
		if i < len(a) {
			temphash[i] = a[i] ^ b[i]
		} else {
			temphash[i] = b[i]
		}
	}

	return temphash
}

// Helper function to marshal Target struct to JSON
func MarshalTarget(t Target) ([]byte, error) {
	jsonData, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// Helper function to unmarshal JSON data to Target struct
func UnmarshalTarget(jsonData []byte) (Target, error) {
	var t Target
	err := json.Unmarshal(jsonData, &t)
	if err != nil {
		return Target{}, err
	}
	return t, nil
}

// Converts a list of converted keys back into the key struct
// Assumes that the byte slice contains only byte iterations of key struct created by the ECP2 ToBytes function
func ToKeys(s []byte, n int) []*BLS12383.ECP2 {
	res := make([]*BLS12383.ECP2, 0)
	if len(s) == 0 || n == 0 {
		return res
	}
	c := len(s) / n
	loop := c
	if len(s)%n != 0 {
		loop += 1
	}

	for i := 0; i < loop; i++ {
		tmp := make([]byte, 0)
		for j := 0; j < n; j++ {
			idx := i*n + j
			if idx == len(s) {
				break
			}
			tmp = append(tmp, s[idx])
		}
		if len(tmp) != 0 {
			res = append(res, BLS12383.ECP2_fromBytes(tmp))
		}
	}
	return res
}

// converts a byte slice into a 2d byte slice, seperated into lengths of n
func To2D(s []byte, n int) [][]byte {
	res := make([][]byte, 0)
	if len(s) == 0 || n == 0 {
		return res
	}
	c := len(s) / n
	loop := c
	if len(s)%n != 0 {
		loop += 1
	}

	for i := 0; i < loop; i++ {
		tmp := make([]byte, 0)
		for j := 0; j < n; j++ {
			idx := i*n + j
			if idx == len(s) {
				break
			}
			tmp = append(tmp, s[idx])
		}
		if len(tmp) != 0 {
			res = append(res, tmp)
		}
	}
	return res
}

// sorts the individual byte slices in a 2d byte slice
func bsort(b [][]byte) [][]byte {
	sort.Slice(b, func(i, j int) bool {
		return bytes.Compare(b[i], b[j]) < 0
	})
	return b
}

// sorts a list of Targets based on the byte slices in the structs
func Tsort(target []Target) []Target {
	sort.Slice(target, func(i, j int) bool {
		return bytes.Compare(target[i].PubKey, target[j].PubKey) < 0
	})
	return target
}

// sorts a slice of AggregatedByes based on the pubkey byte slices
// func AggregateBytesSort(ab []AggregateBytes) []AggregateBytes {
// 	sort.Slice(ab, func(i, j int) bool {
// 		return bytes.Compare(ab[i].Pubkeys[0:97], ab[j].Pubkeys[0:97]) < 0
// 	})
// 	return ab
// }

// sorts a slice of SignatureData based on the
func SignatureDataSort(sd []SignatureData) []SignatureData {
	sort.Slice(sd, func(i, j int) bool {
		return bytes.Compare(sd[i].Target.PubKey[0:97], sd[j].Target.PubKey[0:97]) < 0
	})
	return sd
}

// sorts a CIDList based on the time variable
func CIDListsort(CIDList []CIDData) []CIDData {
	sort.Slice(CIDList, func(i, j int) bool {
		return CIDList[i].time < CIDList[j].time
	})
	return CIDList
}

// Generates FileHistory
func generateFile(id string, ipfs icore.CoreAPI) (FileHistory, cid.Cid) {
	ret := FileHistory{
		OwnerID:        id,
		Message:        []byte("initial file"),
		PreviousCID:    cid.Undef.Bytes(),
		CurrentVersion: 0,
	}
	json, _ := json.Marshal(ret)
	cid, _ := ipfs.Unixfs().Add(context.TODO(), files.NewBytesFile(json), options.Unixfs.CidVersion(1))

	return ret, cid.Cid()
}

// // retrieves the Cid of the sumbitted message
func getByteCid(ipfs icore.CoreAPI, message []byte) cid.Cid {
	cid, _ := ipfs.Unixfs().Add(context.TODO(), files.NewBytesFile(message), options.Unixfs.CidVersion(1))
	return cid.Cid()
}

// Updates a node's IPNS to point towards the input CID
func (vn *VoteNode) ipnsPub(cid path.Resolved) {
	go func(vn *VoteNode, cid path.Resolved) {
		fi, _ := os.OpenFile("./log/ipns.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		defer fi.Close()
		w := bufio.NewWriter(fi)

		namekey := ipns.NameFromPeer(vn.Node.Identity)
		fmt.Fprintf(w, vn.ID+" namekey: "+namekey.String()+"\n")
		w.Flush()

		befpub := time.Now().Unix() / Joininterval
		name, err := vn.IPFS.Name().Publish(Loopctx, cid)
		if err != nil {
			panic(fmt.Errorf("failed to create IPNS name: %s", err))
		}
		fmt.Fprintf(w, vn.ID+" name: "+name.String()+"\n")
		afpub := time.Now().Unix() / Joininterval
		fmt.Fprintf(w, vn.ID+": before: "+strconv.FormatInt(befpub, 10)+" after: "+strconv.FormatInt(afpub, 10)+"\n")
		namekeycid, err := vn.IPFS.Name().Resolve(ctx, name.String())
		if err != nil {
			panic(fmt.Errorf("resolve name failed: %s", err))
		}
		resolved, err := vn.IPFS.ResolvePath(ctx, namekeycid)
		if err != nil {
			panic(fmt.Errorf("resolve path failed: %s", err))
		}
		fmt.Fprintf(w, vn.ID+" name: "+resolved.Cid().String()+" versus CID: "+vn.CurrentCID.String()+"\n")

		vn.Name = name
		w.Flush()
	}(vn, cid)
}

// returns the CID of the latest block's information
func (vn *VoteNode) getLatestBlock() []byte {
	if vn.ChainAddress == "" {
		return make([]byte, 0)
	}
	client := &http.Client{}
	var data = strings.NewReader(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", true],"id":1}`)
	req, err := http.NewRequest("POST", vn.ChainAddress, data)
	if err != nil {
		// fmt.Errorf("invalid http request", err)
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	logging(vn, "latestblock: "+string(bodyText))

	block := generateBlockStruct()
	err = json.Unmarshal(bodyText, &block)
	if err != nil {
		// fmt.Errorf("unmarshal error", err)
		log.Fatal(err)
	}

	// logging(vn, "block " + )
	logging(vn, "block x: "+block.JSONRPC)
	logging(vn, "block hash "+block.Result.Hash)
	modifiedbytes, err := hex.DecodeString(block.Result.Hash[2:])
	if err != nil {
		// fmt.Errorf("decode error", err)
		log.Fatal(err)
	}
	logging(vn, "block hash modified: "+string(modifiedbytes))
	logging(vn, "modified length: "+strconv.Itoa(len(modifiedbytes)))

	// H := miracl.NewHASH256()
	// H.Process_array(bodyText)

	// ret, err := vn.IPFS.Unixfs().Add(context.TODO(), files.NewBytesFile(bodyText), options.Unixfs.CidVersion(1))
	vn.msgTmp = modifiedbytes //change to string
	return modifiedbytes      //change to string
	// return bodyText
	// return getByteCid(vn.IPFS, bodyText).Bytes()
}

// generates a default EthereumBlock struct
func generateBlockStruct() EthereumBlock {
	result := BlockResult{
		Number:           "temp",
		Hash:             "temp",
		MixHash:          "temp",
		ParentHash:       "temp",
		Nonce:            "temp",
		Sha3Uncles:       "temp",
		LogsBloom:        "temp",
		TransactionsRoot: "temp",
		StateRoot:        "temp",
		ReceiptsRoot:     "temp",
		Miner:            "temp",
		Difficulty:       "temp",
		TotalDifficulty:  "temp",
		ExtraData:        "temp",
		Size:             "temp",
		GasLimit:         "temp",
		GasUsed:          "temp",
		Timestamp:        "temp",
		Uncles:           make([]string, 0),
		Transactions:     make([]string, 0),
	}

	ret := EthereumBlock{
		JSONRPC: "temp",
		ID:      0,
		Result:  &result,
	}
	return ret
}

// Creates a Target using the latest block
func (vn *VoteNode) CreateTarget() Target {
	key := vn.Key2
	// tempfh := vn.CurrentFile //FileHistory
	var temphash []byte
	message := vn.getLatestBlock()

	if len(message) == 0 { //change to check if message == previous?
		temphash = vn.CurrentCID.Bytes()
		logging(vn, "empty message!")
	} else {
		// b := getMessageCid(vn.IPFS, []byte(tempfh.Message)).Bytes()
		b := vn.CurrentCID.Bytes()

		logging(vn, "lengths of b and message: "+strconv.Itoa(len(b))+"and "+strconv.Itoa(len(message)))
		temphash = byteXOR(b, message)
		logging(vn, "length of temphash: "+strconv.Itoa(len(temphash)))
		logging(vn, "entered else, here is XOR: "+(string(temphash)))
		// temphash = append([]byte(tempfh.Message), vn.CurrentCID.Bytes()...) //TO DO: change message to CID, then XOR message CID with current CID
	}
	//TO DO: HASH the temphash
	keyByte := make([]byte, 97)
	key.ToBytes(keyByte, true)

	return Target{
		Name:   vn.Name.String(),
		Hash:   temphash,
		PubKey: keyByte,
	}
}

// creates a generic target struct for the sake of testing
func (vn *VoteNode) GenericTarget() (Target, *BLS12383.BIG) {
	_, key, skey := genKey()

	temphash := make([]byte, 64)

	_, err := rand.Read(temphash)
	if err != nil {
		panic(err)
	}

	//TO DO: HASH the temphash
	keyByte := make([]byte, 97)
	key.ToBytes(keyByte, true)

	return Target{
		Hash:   temphash,
		PubKey: keyByte,
	}, skey
}

// Helper function to marshal FileHistory struct to JSON
func MarshalSignature(s SignatureData) ([]byte, error) {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// Helper function to unmarshal JSON data to FileHistory struct
func UnmarshalSignature(jsonData []byte) (SignatureData, error) {
	var s SignatureData
	err := json.Unmarshal(jsonData, &s)
	if err != nil {
		target := Target{
			Hash:   []byte{},
			PubKey: []byte{},
		}
		ret := SignatureData{
			SignedHash: []byte{},
			Target:     target,
		}
		return ret, err
	}
	return s, nil
}

// converts a list of pubkeys into []byte, and then concatenates them into a singular []byte
func PubKeysToBytes(pks []*BLS12383.ECP2) []byte {
	byteSlice := make([]byte, 0)
	for _, item := range pks {
		// fmt.Println("index: " + strconv.Itoa(i))
		// fmt.Println("itemTyte: " + reflect.TypeOf(item).String())
		// fmt.Println("item: " + item.ToString())
		tempByteSlice := make([]byte, 200)
		item.ToBytes(tempByteSlice, false)
		byteSlice = append(byteSlice, tempByteSlice...)
	}
	return byteSlice
}

// saves Aggregate Data into a FileHistory struct, and writes to file
func (vn *VoteNode) saveAggregateData(ut string, a AggregateBytes) cid.Cid {
	// fmt.Println("saveAggregateData: ", a)
	pcid := vn.CurrentCID.Bytes()
	msg := vn.msgTmp
	cv := vn.CurrentFile.CurrentVersion + 1
	if vn.FirstPublish {
		pcid = cid.Undef.Bytes()
		msg = []byte("initial file")
		vn.FirstPublish = false
		cv = 0
	}
	// hash := bytes.Join(a.messages, []byte{})
	// fmt.Println("messages hash: ", hash)

	// tab := convertBoolToByte(a.table)
	// // fmt.Println("table: ", tab)

	// aggbytes := a
	aByte, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}

	abytes := FileHistory{
		OwnerID:        vn.ID,
		Message:        msg,
		CurrentVersion: cv,
		PreviousCID:    pcid,
		AggregateBytes: aByte,
	}
	// vn.ipnsPub(abytes)
	vn.CurrentFile = abytes
	// fmt.Println("a: ", a)
	// fmt.Println("aByte: ", aByte)

	fmt.Println("vn : ", vn)
	fmt.Println("logfile : ", vn.LogFile)
	fmt.Println("logfile name : ", vn.LogFile.Name())

	f, _ := os.OpenFile(vn.LogFile.Name()+"_agg_"+ut+".log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	defer f.Close()
	jsonData, err := json.Marshal(abytes)
	if err != nil {
		panic(err)
	}
	// fmt.Println("agregate jsonData: ", jsonData)
	// fmt.Println(jsonData)
	if _, err := f.Write(jsonData); err != nil {
		panic(err)
	}

	fhcid, _ := vn.IPFS.Unixfs().Add(context.TODO(), files.NewBytesFile(jsonData), options.Unixfs.CidVersion(1))

	// json, _ := MarshalFileHistory(abytes)
	// fmt.Println("agregate json: ", string(json))
	// // fmt.Println(jsonData)

	for j := 0; j < len(vn.CIDList); j++ {
		listCID := vn.CIDList[j]
		logging(vn, "CIDList index: "+strconv.Itoa(j))
		logging(vn, "CIDList cid: "+string(listCID.CID))
		logging(vn, "CIDList time: "+strconv.FormatInt(listCID.time, 10))
	}

	byteCIDs, err := json.Marshal(vn.CIDList)
	if err != nil {
		panic(err)
	}
	newks := ChainHead{
		Keys:        vn.KeyList,
		Names:       vn.NameList,
		Filehistory: fhcid.Cid().Bytes(),
		CIDList:     byteCIDs,
	}

	fmt.Println("filehistory CID in keylist: ", newks.Filehistory)

	jsonName, err := json.Marshal(newks)
	if err != nil {
		panic(err)
	}

	if _, err := f.Write(jsonName); err != nil {
		panic(err)
	}

	// var unmarshalkl KeyList
	// err = json.Unmarshal(jsonName, &unmarshalkl)
	// if err != nil {
	// 	panic(fmt.Errorf("keylist unmarshal error: %s", err))
	// }

	// // startFH := unmarshalkl.Filehistory
	// tempList := unmarshalkl.CIDList
	// for j := 0; j < len(tempList); j++ {
	// 	listCID := tempList[j]
	// 	logging(vn, "CIDList index: "+strconv.Itoa(j))
	// 	logging(vn, "CIDList cid: "+string(listCID.CID))
	// 	logging(vn, "CIDList time: "+strconv.FormatInt(listCID.time, 10))
	// }

	cid, _ := vn.IPFS.Unixfs().Add(context.TODO(), files.NewBytesFile(jsonName), options.Unixfs.CidVersion(1))
	vn.ipnsPub(cid)
	fmt.Println("printed", jsonName)

	return cid.Cid()
}

// Unmarshals the aggregate bytes in a FileHistory
func FileHistory2AggregateData(f FileHistory) AggregateBytes {
	// fmt.Println("fileHistory2AggregateData: ", f)
	var ab AggregateBytes
	err := json.Unmarshal(f.AggregateBytes, &ab)
	if err != nil {
		panic(err)
	}
	return ab
}

// Adds a key and name to a VoteNode's keylist and namelist
func (vn *VoteNode) AddKeyName(key []byte, name string) {
	vn.KeyList = append(vn.KeyList, key)
	vn.NameList = append(vn.NameList, name)
}
