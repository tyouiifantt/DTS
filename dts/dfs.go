package dts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-cid"
)

// TO DO
// The functions below are called in test.go assuming a target block and verifier name has already been chosen. A function needs to be created to pull this from file.
// the returned CIDData is likely insufficient for tracing the verification path; this needs to be improved
// The functions were created as such because I was unsure about updating a list while we were iterating through a list. While the current implementation should work, it could be improved to more closely match the conventional DFS structure.

// checkNeighbors goes through the blocks in a subchain, and checks each blocks peers to see if the targetname is one of them
// if so, it returns true and the CIDData
// if false, it creates a list of NameTime using the peers and the unixtime of the block, and call checklist
func checkNeighbors(subchain SubChain, targetname string, visited []string, deadline int64) (bool, []CIDData) {
	ret := make([]NameTime, 0)
	sc := subchain.subchain
	namelist := subchain.names
	for _, block := range sc {
		_, cid, err := cid.CidFromBytes(block.CID)
		if err != nil {
			panic(err)
		}
		node, err := fieldipfs.Dag().Get(context.TODO(), cid)
		if err != nil {
			panic(err)
		}
		jsonfile := node.RawData()
		var unmarshalfh FileHistory
		err = json.Unmarshal(jsonfile, &unmarshalfh)
		if err != nil {
			panic(err)
		}
		var unmarshalab AggregateBytes
		err = json.Unmarshal(unmarshalfh.AggregateBytes, &unmarshalab)
		if err != nil {
			panic(err)
		}
		names := retrieveNames(namelist, unmarshalab.Indexes)
		checklist := containsString(names, targetname)
		if checklist {
			return true, []CIDData{block}
		} else {
			for _, name := range names {
				if !containsString(visited, name) {
					nametime := NameTime{
						Name: name,
						time: block.time,
					}
					ret = append(ret, nametime)
				}
			}
		}
	}
	visited = append(visited, subchain.name)
	ret = filterNames(ret, visited)
	// temp, path :=
	return checklist(ret, targetname, visited, deadline)

}

// checklist goes through the list of NameTime, creating a subchain for each one and checking neighbors recursively
func checklist(ntl []NameTime, targetname string, visited []string, deadline int64) (bool, []CIDData) {
	if len(ntl) == 0 {
		return false, make([]CIDData, 0)
	}
	for _, nt := range ntl {
		curSB := createSubchainFromName(nt, deadline)
		temp, tempCID := checkNeighbors(curSB, targetname, visited, deadline)
		if temp {
			return temp, tempCID
		}
	}
	return false, make([]CIDData, 0)
}

func popFirstNT(slice []NameTime) (NameTime, []NameTime) {
	if len(slice) == 0 {
		return NameTime{}, slice
	}
	first := slice[0]
	slice = slice[1:]
	return first, slice
}

// Takes a list of NameTime, and removes all variables that contain a Name that is already in visited
func filterNames(names []NameTime, visited []string) []NameTime {
	ret := make([]NameTime, 0)
	for _, nt := range names {
		if !containsString(visited, nt.Name) {
			ret = append(ret, nt)
		}
	}
	return ret
}

// Creates a subchain from a NameTime, which is a IPNS Name and unix time.
// The subchain should be all blocks that exist between the unix time stored, and the deadline
func createSubchainFromName(nt NameTime, deadline int64) SubChain {
	jsonfile := NameParse(nt.Name)
	fmt.Println(jsonfile)
	var unmarshalkl ChainHead
	err := json.Unmarshal(jsonfile, &unmarshalkl)
	if err != nil {
		panic(fmt.Errorf("keylist unmarshal error: %s", err))
	}
	var unmarshallist []CIDData
	err = json.Unmarshal(unmarshalkl.CIDList, &unmarshallist)

	mainchain := CIDListsort(unmarshallist)
	var sc []CIDData
	for i := 0; i < len(mainchain); i++ {
		if nt.time <= mainchain[i].time && mainchain[i].time <= deadline {
			sc = append(sc, mainchain[i])
		}
	}

	return SubChain{sc, unmarshalkl.Names, nt.Name}
}
