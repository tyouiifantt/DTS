package dts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	iface "github.com/ipfs/boxo/coreiface"
	"github.com/ipfs/go-cid"
)

var fieldipfs iface.CoreAPI

// var interval = 3
var sigs = 0

func TestIPFS(t *testing.T) {
	// fmt.Println("starting test")
	fieldipfs, _ = StartNode(false, true)

	ct, cancel := context.WithCancel(context.Background())
	defer cancel()
	fmt.Println(ct)

	// loopctx = ct
	// vn := NewVoteNode(0, fieldipfs, retnode)
	//reads IPNS from file
	//retrieve list of CIDs from each IPNS
	//run trace on each CID
	tt := time.Now()
	nodes := ReadNodes(true)
	sigst := 0
	for i, v := range nodes {
		sigst += sigs
		sigs = 0
		//時刻を記録
		t := time.Now()
		fmt.Println("node #", i, " address: ", v)
		subv := v[strings.LastIndex(v, "/")+1:]
		fmt.Println("name key: ", subv+"\n \n \n \n")
		//go to IPNS and retrieve CID
		jsonfile := NameParse(subv)
		fmt.Println(jsonfile)

		var unmarshalkl ChainHead
		err := json.Unmarshal(jsonfile, &unmarshalkl)
		if err != nil {
			panic(fmt.Errorf("keylist unmarshal error: %s", err))
		}

		startFH := unmarshalkl.Filehistory
		var tempList []CIDData
		err = json.Unmarshal(unmarshalkl.CIDList, &tempList)
		if err != nil {
			panic(fmt.Errorf("CIDData unmarshal error: %s", err))
		}

		for j := 0; j < len(tempList); j++ {
			listCID := tempList[j]
			fmt.Println("CIDList index: " + strconv.Itoa(j))
			fmt.Println("CIDList cid: " + string(listCID.CID))
			fmt.Println("CIDList time: " + strconv.FormatInt(listCID.time, 10))
		}

		_, retcid, err := cid.CidFromBytes(startFH)
		if err != nil {
			fmt.Println(err)
		}

		TraceCID(unmarshalkl.Keys, retcid)
		fmt.Println("time: ", time.Since(t), " for ", sigs, " signatures")
		fmt.Println("timetotal : ", time.Since(tt), " for total ", sigst, " signatures")
	}

	// t.Log("hellow world")

}

// Verifies the aggregate signature
func VerifyAgg(keylist [][]byte, fh FileHistory) bool {
	f, err := os.OpenFile("test.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Errorf("file creation error"))
	}
	defer f.Close()

	agg := FileHistory2AggregateData(fh)

	// fmt.Println("AggregateData: ", agg)

	flag := false
	checkXOR := byteXOR(fh.Message, fh.PreviousCID)
	if TestVerifyAggregation(keylist, agg, checkXOR) {
		flag = true
	}
	fmt.Println("test result: " + strconv.FormatBool(flag))
	return flag
}

// Traces the FileHistory chain all the way to the first FileHistory
func TraceCID(keylist [][]byte, tcid cid.Cid) bool {
	sigs++
	fmt.Println("TraceCID: ", tcid.String())
	node, err := fieldipfs.Dag().Get(context.TODO(), tcid)
	jsonfile := node.RawData()
	if err != nil {
		return false
	}
	fmt.Println(jsonfile)

	// var unmarshalkl KeyList
	// err = json.Unmarshal(jsonfile, &unmarshalkl)
	// if err != nil {
	// 	panic(fmt.Errorf("keylist unmarshal error: %s", err))
	// }

	var unmarshalfh FileHistory
	err = json.Unmarshal(jsonfile, &unmarshalfh)
	if err != nil {
		panic(fmt.Errorf("unmarshal error: %s", err))
	}
	fmt.Println("json: ", jsonfile)
	fmt.Println("FileHistory: ", unmarshalfh)
	fmt.Println("Unmarshaled FileHistory CV: ", unmarshalfh.CurrentVersion)

	// var unmarshalfh FileHistory
	// err = json.Unmarshal(unmarshalkl.Filehistory, &unmarshalfh)
	// if err != nil {
	// 	panic(fmt.Errorf("filehistory unmarshal error: %s", err))
	// }

	// if unmarshalfh.CurrentVersion == 1  {
	// 	return true
	// }
	if unmarshalfh.CurrentVersion == 0 {
		return true
	}

	vg := VerifyAgg(keylist, unmarshalfh)
	fmt.Println("VerifyAgg: ", vg)
	if !vg {
		return false
	}
	// VerifyAggregation(unmarshalfh)

	previous := unmarshalfh.PreviousCID
	if string(previous) == string(cid.Undef.Bytes()) {
		return true
	} else {
		_, retcid, err := cid.CidFromBytes(previous)
		if err != nil {
			fmt.Println(err)
		}
		return TraceCID(keylist, retcid)
	}

	// return false
}

// starts the DFS process given a block, IPNS name that the block belongs to, a verifier name, and a time constraint
func startDFS(block cid.Cid, name string, targetname string, timeconstraint int64) bool {

	subchain, deadline := createSubchainFromBlock(block, name, timeconstraint)

	ret, blocklist := checkNeighbors(subchain, targetname, []string{name}, deadline) //checks each block in subchain starting from root

	fmt.Println(blocklist)

	return ret
}

// creates a subchain from a block
func createSubchainFromBlock(block cid.Cid, name string, timeconstraint int64) (SubChain, int64) {
	jsonfile := NameParse(name)
	fmt.Println(jsonfile)
	var unmarshalkl ChainHead
	err := json.Unmarshal(jsonfile, &unmarshalkl)
	if err != nil {
		panic(fmt.Errorf("keylist unmarshal error: %s", err))
	}
	var unmarshallist []CIDData
	err = json.Unmarshal(unmarshalkl.CIDList, &unmarshallist)
	if err != nil {
		panic(fmt.Errorf("CIDList unmarshal error: %s", err))
	}

	mainchain := CIDListsort(unmarshallist)
	blockindex := FindIndexByData(mainchain, block.Bytes())
	starttime := mainchain[blockindex].time
	deadline := starttime + timeconstraint
	var sc []CIDData
	for i := blockindex; i < len(mainchain); i++ {
		if starttime <= mainchain[i].time && mainchain[i].time <= deadline {
			sc = append(sc, mainchain[i])
		}
	}

	return SubChain{sc, unmarshalkl.Names, name}, deadline
}

// Takes an IPNS name and returns the raw data in a []byte
func NameParse(name string) []byte {
	namekeycid, err := fieldipfs.Name().Resolve(context.TODO(), name)
	if err != nil {
		fmt.Errorf("failed to retrieve cid")
	}
	fmt.Println("namekeycid:  ", namekeycid)
	if namekeycid == nil {
		panic("Null namekeyCID: try again later")
	}
	resolved, err := fieldipfs.ResolvePath(context.TODO(), namekeycid)
	if err != nil {
		panic(fmt.Errorf("resolve path failed: %s", err))
	}
	node, err := fieldipfs.Dag().Get(context.TODO(), resolved.Cid())
	jsonfile := node.RawData()
	if err != nil {
		panic(fmt.Errorf("failed to get KeyList: %s", err))
	}
	return jsonfile
}

// returns the index of the CIDData containing data if it exists, returns -1 if it doesn't
func FindIndexByData(list []CIDData, data []byte) int {
	for i, cid := range list {
		if bytesEqual(cid.CID, data) {
			return i
		}
	}
	return -1 // Return -1 if not found
}

// checks if 2 byte slices are the same
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// checks if a []string contains a certain string
func containsString(array []string, target string) bool {
	for _, str := range array {
		if str == target {
			return true
		}
	}
	return false
}

// func ReadAggregateData() []AggregateData {
// 	file, err := os.Open("Aggregated Data.log")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer file.Close()

// 	var dataslice []AggregateData
// 	for {
// 		var databyte AggregateBytes
// 		_, err := fmt.Fscanln(file, &databyte)
// 		if err != nil {
// 			break
// 		}
// 		dataslice = append(dataslice)
// 	}

// 	return dataslice
// }

// func readFromFile(a AggregateBytes) AggregateData {
// 	fmt.Print("beginning of saveToFile(): ")
// 	fmt.Println(a)

// 	sigma := BLS12383.ECP_fromBytes(a.sigmaagg)
// 	fmt.Print("sigma: ")
// 	fmt.Println(sigma)

// 	pks := ToKeys(a.pubkeys, 97)

// 	adata := AggregateData{
// 		sigmaagg: sigma,
// 		messages: To2D(a.messages, 40),
// 		pubkeys:  pks,
// 		table:    convertToBool(a.table),
// 	}

// 	return adata
// }

// func readNodes() []string {
// 	file, err := os.Open("../nodes.log.txt")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer file.Close()

// 	var nodes []string
// 	for {
// 		var node string
// 		_, err := fmt.Fscanln(file, &node)
// 		if err != nil {
// 			break
// 		}
// 		nodes = append(nodes, node)
// 	}
// 	return nodes
// }
