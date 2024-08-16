package dts

// package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	miracl "miracl/core"
	"miracl/core/BLS12383"
	"os"
	"strconv"
	"strings"
	"time"

	icore "github.com/ipfs/boxo/coreiface"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/kubo/core"
)

// creates a new VoteNode struct
func NewVoteNode(i int, ipfs icore.CoreAPI, node *core.IpfsNode, address string) *VoteNode {
	id := fmt.Sprintf("%03d", i)

	// f, err := os.OpenFile(id+".log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	panic(fmt.Errorf("file creation error"))
	// }
	// defer f.Close()

	cf, cid := generateFile(id, ipfs)

	n := ipns.NameFromPeer(node.Identity)
	//fmt.Println("key created: " + n.String())

	// namekey := ipns.NameFromPeer(node.Identity)
	//fmt.Println("nodeID: " + node.Identity.ShortString())

	k1, k2, ks := genKey()

	logfile, err := os.OpenFile("./log/"+node.Identity.String()+".log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err)
	}
	multiLogFile := io.MultiWriter(os.Stdout, logfile)
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	log.SetOutput(multiLogFile)

	ret := &VoteNode{
		ID:          id,
		IPFS:        ipfs,
		Node:        node,
		CurrentFile: cf,
		CurrentCID:  cid,
		// logWriter:    logw,
		LogFile:      logfile,
		Name:         n,
		FirstPublish: true,
		Key1:         k1,
		Key2:         k2,
		KeyS:         ks,
		msgTmp:       nil,
		ChainAddress: address,
	}
	return ret
}

// starts a joint signature
func (vn *VoteNode) Vote(t time.Time, i int, tn bool) []byte {

	tu := time.Now().Unix() / Joininterval

	logging(vn, vn.ID+":loop")
	namekey := ipns.NameFromPeer(vn.Node.Identity)
	logging(vn, ":voting "+namekey.String())

	if tu%7 != 0 {
		logging(vn, vn.ID+": entered wrong at "+strconv.FormatInt(tu, 10))
		return nil
	}

	var ret []byte

	if tn { //for testing purposes only
		ret = vn.MassPublish(tu)
	} else {
		ret = vn.JointSignature(tu)
	}

	return ret
}

// Used for testing MassPublish; not intended for practical cases
func (vn *VoteNode) MassPublish(tu int64) []byte {
	beginningtime := time.Now()

	// create unixtime variable
	unixtime := strconv.FormatInt(tu, 10)
	checktime := strconv.FormatInt(time.Now().Unix()/Joininterval, 10)

	// //allows problem nodes to join properly, but does not prevent skipping timestamps
	if unixtime != checktime {
		logging(vn, "checktime and unixtime mismatch")
	}

	targets, skeys := vn.pubTargets(tu, beginningtime)
	// logging(vn, "targets received at "+unixtime+": "+strconv.Itoa(len(targets)))
	// targets = Tsort(targets)

	var sigmas []SignatureData

	// Signing portion
	for i := 0; i < len(skeys); i++ {
		sigma := TestSigning(vn, targets, skeys[i])
		sigmas = append(sigmas, sigma)
	}

	// logging(vn, "sign target Pubkey length : "+strconv.Itoa(len(sigma.Target.PubKey)/97))
	// // logging(vn, "sigma sig: "+string(sigma.SignedHash))
	// // logging(vn, "sigma conv: "+string(sigma.Target.Hash))

	vn.pubSigs(tu, beginningtime, sigmas)
	// logging(vn, "received at "+unixtime+": "+strconv.FormatInt(int64(iterations), 10))

	// logging(vn, "Sig Aggregation saved to :"+cid.String())
	// vn.CurrentCID = cid

	return nil
}

// VoteNode joint-signs with peers to create a joint signature
func (vn *VoteNode) JointSignature(tu int64) []byte {
	beginningtime := time.Now()

	// create unixtime variable
	unixtime := strconv.FormatInt(tu, 10)
	// logging(vn, "unixtime: "+unixtime)
	checktime := strconv.FormatInt(time.Now().Unix()/Joininterval, 10)
	// logging(vn, "checktime: "+checktime)

	// //allows problem nodes to join properly, but does not prevent skipping timestamps
	if unixtime != checktime {
		logging(vn, "checktime and unixtime mismatch")
	}

	targets := vn.pubsubTargets(tu, beginningtime)
	logging(vn, "targets received at "+unixtime+": "+strconv.Itoa(len(targets)))
	targets = Tsort(targets)

	// Signing portion
	sigma := Signing(vn, targets)
	logging(vn, "sign target Pubkey length : "+strconv.Itoa(len(sigma.Target.PubKey)/97))
	// logging(vn, "sigma sig: "+string(sigma.SignedHash))
	// logging(vn, "sigma conv: "+string(sigma.Target.Hash))

	iterations, receivedSignatures := vn.pubsubSigs(tu, beginningtime, sigma)
	logging(vn, "received at "+unixtime+": "+strconv.FormatInt(int64(iterations), 10))

	sigmaagg := Aggregate(vn, receivedSignatures)

	// fmt.Println("about to print size")
	// fmt.Println(unsafe.Sizeof(sigmaagg))
	// fmt.Println(unsafe.Sizeof(sigmaagg.Messages))
	// fmt.Println(unsafe.Sizeof(sigmaagg.Pubkeys))
	// fmt.Println(unsafe.Sizeof(sigmaagg.Sigmaagg))
	// fmt.Println(unsafe.Sizeof(sigmaagg.Table))

	logging(vn, "Sig Aggregation finished")
	verif := VerifyAggregation(vn, sigmaagg)
	logging(vn, "verify Aggregation sig: "+strconv.FormatBool(verif))
	cid := vn.saveAggregateData(unixtime, sigmaagg)

	logging(vn, "Sig Aggregation saved to :"+cid.String())
	logging(vn, "tu right before saving: "+strconv.FormatInt(tu, 10))

	vn.CurrentCID = cid
	tempciddata := CIDData{
		CID:  cid.Bytes(),
		time: tu,
	}

	vn.CIDList = append(vn.CIDList, tempciddata)

	return cid.Bytes()
}

// Signing function
// combines keys and hashes obtained via pubsub per paper specifications
func Signing(vn *VoteNode, targets []Target) SignatureData {
	var combinedhash []byte
	var pubkeys []byte
	var namearray []string
	var names string
	keyByte := make([]byte, 97)
	vn.Key2.ToBytes(keyByte, true)

	for i := 0; i < len(targets); i++ {
		// logging(vn, "message length: "+strconv.Itoa(len(targets[i].Hash)))
		combinedhash = append(combinedhash, targets[i].Hash...) //possibly change to xor
		pubkeys = append(pubkeys, targets[i].PubKey...)
		namearray = append(namearray, targets[i].Name)
	}
	names = strings.Join(namearray, "")
	pubkeys = append(keyByte, pubkeys...)

	logging(vn, "combinedhash: "+strconv.Itoa(len(combinedhash)))

	sigTarget := append(combinedhash, keyByte...)
	logging(vn, "sigTarget: "+strconv.Itoa(len(sigTarget)))

	signedhash := sign(sigTarget, *vn.KeyS)
	sigByte := make([]byte, 49)
	signedhash.ToBytes(sigByte, true)

	target := Target{
		Name:   names,
		Hash:   combinedhash,
		PubKey: pubkeys,
	}
	sigma := SignatureData{
		SignedHash: sigByte,
		Target:     target,
	}

	return sigma
}

// Alternate signing function for Mass Publish testing
func TestSigning(vn *VoteNode, targets []Target, skey *BLS12383.BIG) SignatureData {
	var combinedhash []byte
	var pubkeys []byte
	keyByte := make([]byte, 97)
	vn.Key2.ToBytes(keyByte, true)

	for i := 0; i < len(targets); i++ {
		combinedhash = append(combinedhash, targets[i].Hash...) //possibly change to xor
		pubkeys = append(pubkeys, targets[i].PubKey...)
	}
	pubkeys = append(keyByte, pubkeys...)

	logging(vn, "combinedhash: "+strconv.Itoa(len(combinedhash)))

	sigTarget := append(combinedhash, keyByte...)
	logging(vn, "sigTarget: "+strconv.Itoa(len(sigTarget)))

	signedhash := sign(sigTarget, *skey)
	sigByte := make([]byte, 49)
	signedhash.ToBytes(sigByte, true)

	target := Target{
		Hash:   combinedhash,
		PubKey: pubkeys,
	}
	sigma := SignatureData{
		SignedHash: sigByte,
		Target:     target,
	}

	return sigma
}

// Aggregate function according to paper
func Aggregate(vn *VoteNode, s []SignatureData) AggregateBytes {
	s = SignatureDataSort(s)
	first := true
	sigmaagg := BLS12383.ECP_generator()
	messages := make([]byte, 0)
	targetPubkeys := make([][]byte, 0)
	size := 0
	isVerified := make([]bool, 0)
	table := make([][]bool, 0)
	targets := make([]Target, 0)
	indexes := make([]int, 0)
	names := make([]string, 0)

	// this for loop iterates through all lists, and creates new lists that remove all duplicates
	for i := 0; i < len(s); i++ {
		targets = append(targets, s[i].Target)
		tempmessages := To2D(targets[i].Hash, 36)
		tempnames := stringbreak(targets[i].Name, 56)
		new := true
		for k := 0; k < len(s[i].Target.PubKey)/97-1; k++ {
			for j := 0; j < len(targetPubkeys); j++ {
				if bytes.Equal(targetPubkeys[j], s[i].Target.PubKey[(k+1)*97:(k+2)*97]) {
					new = false
					break
				}
			}
			// conbool, index := contains(vn.KeyList, s[i].Target.PubKey[(k+1)*97:(k+2)*97])
			if new {
				// vn.AddKey(s[i].Target.PubKey[(k+1)*97 : (k+2)*97])
				targetPubkeys = append(targetPubkeys, s[i].Target.PubKey[(k+1)*97:(k+2)*97])
				messages = append(messages, tempmessages[k]...)
				names = append(names, tempnames[k])
				// _, index = contains(vn.KeyList, s[i].Target.PubKey[(k+1)*97:(k+2)*97])
				// indexes = append(indexes, index)
			}
		}
	}
	// signerPubkeys = bsort(signerPubkeys)  //for testing mainly
	targetPubkeys = bsort(targetPubkeys)

	// verifies the signature for each target
	for i := 0; i < len(targets); i++ {
		pk := BLS12383.ECP2_fromBytes(targets[i].PubKey[0:97])
		// logging(vn, "pk: "+pk.ToString())
		sig := BLS12383.ECP_fromBytes(s[i].SignedHash)
		sigTarget := append(targets[i].Hash, targets[i].PubKey[0:97]...)
		// logging(vn, "verifTarget: "+strconv.Itoa(len(sigTarget))+" : "+string(sigTarget))
		if verify(sig, pk, sigTarget) {
			isVerified = append(isVerified, true)
			size++
			// logging(vn, "signature verified")
			if first {
				first = false
				sigmaagg = sig
			} else {
				sigmaagg.Add(sig)
			}
		} else {
			isVerified = append(isVerified, false)
			// logging(vn, "signature not verified")
		}
	}
	logging(vn, "signature verified : "+strconv.Itoa(len(isVerified))+" unverified : "+strconv.Itoa(len(targets)-len(isVerified)))

	sigByte := make([]byte, 49)
	sigmaagg.ToBytes(sigByte, true)

	// creates the base table
	for i := 0; i < size; i++ {
		minitable := make([]bool, 0)
		for j := 0; j < len(targetPubkeys); j++ {
			minitable = append(minitable, false)
		}
		table = append(table, minitable)
	}

	index := 0
	//fills out the table based on the appearence of keys
	for i := 0; i < len(isVerified); i++ {
		if isVerified[i] {
			for j := 0; j < len(targetPubkeys); j++ {
				new := false
				tPk := To2D(targets[i].PubKey, 97)

				for k := 0; k < len(tPk); k++ {
					if bytes.Equal(targetPubkeys[j], tPk[k]) {
						new = true
						break
					}
				}
				if new {
					table[index][j] = true
				} else {
					table[index][j] = false
				}
			}
			index++
		}
	}

	// conbool, index := contains(vn.KeyList, s[i].Target.PubKey[(k+1)*97:(k+2)*97])
	for i := 0; i < len(targetPubkeys); i++ {
		conbool, index := contains(vn.KeyList, targetPubkeys[i])
		if !conbool {
			vn.AddKeyName(targetPubkeys[i], names[i])
			_, index = contains(vn.KeyList, targetPubkeys[i])

		}
		indexes = append(indexes, index)
	}
	fmt.Println("targetpubkeys:", targetPubkeys)
	fmt.Println("indexes: ", indexes)
	// pubkeys := make([]byte, 0)
	// for i := 0; i < len(targetPubkeys); i++ {
	// 	pubkeys = append(pubkeys, targetPubkeys[i]...)
	// }

	fmt.Print("table", table)
	// bitlength, bittable := tableToBits(table)
	rows, bittable := tableToBits(table)
	fmt.Print("table after conversion", bittable)

	// fmt.Println("printing sizes in aggregate")
	// fmt.Println(unsafe.Sizeof(sigByte))
	// fmt.Println(unsafe.Sizeof(messages))
	// fmt.Println(unsafe.Sizeof(pubkeys))
	// fmt.Println(unsafe.Sizeof(table))

	// fmt.Println(reflect.Type.Size(sigByte))

	return AggregateBytes{Sigmaagg: sigByte, Messages: messages, Indexes: indexes, Table: bittable, Rows: rows}
}

// Verifies an aggregated signature
func VerifyAggregation(vn *VoteNode, ad AggregateBytes) bool {
	sigmaagg := ad.Sigmaagg
	pubkeys := retrieveKeys(vn.KeyList, ad.Indexes)
	messages := ad.Messages
	// table := bitsToTable(ad.Rows, ad.Table)
	table := bitsToTable(ad.Rows, ad.Table)

	if len(pubkeys)/97 != len(messages)/36 {
		logging(vn, "mismatching keys and messages")
		return false
	}
	logging(vn, "keys and table match!")
	if len(pubkeys)/97 != len(table) {
		logging(vn, "mismatching keys and table")
		return false
	}
	logging(vn, "keys and table match!")

	eleft := BLS12383.NewFP12int(1)

	checkHash := byteXOR(vn.msgTmp, vn.CurrentCID.Bytes())

	for i := 0; i < len(pubkeys)/97; i++ {
		currentrow := table[i]
		var currenthash []byte
		tempkey := pubkeys[i*97 : (i+1)*97]
		for j := 0; j < len(pubkeys)/97; j++ {
			if currentrow[j] {
				currenthash = append(currenthash, messages[j*36:(j+1)*36]...)
			}
		}
		currenthash = append(currenthash, tempkey...)

		cont := bytes.Contains(currenthash, checkHash)
		if !cont {
			return false
		}

		logging(vn, "verifyTarget: "+strconv.Itoa(len(currenthash))+" : "+string(currenthash))

		pubkey := BLS12383.ECP2_fromBytes(pubkeys[i*97 : (i+1)*97])
		H := miracl.NewHASH256()
		H.Process_array(currenthash)
		Hm := BLS12383.BLS_hash_to_point(H.Hash())
		ei := BLS12383.Ate(pubkey, Hm)
		ei = BLS12383.Fexp(ei)
		// logging(vn, "ei:"+ei.ToString())
		eleft.Mul(ei)
	}
	sigP := BLS12383.ECP_fromBytes(sigmaagg)
	eright := BLS12383.Ate(BLS12383.ECP2_generator(), sigP)
	eright = BLS12383.Fexp(eright)

	logging(vn, "el:"+eleft.ToString())
	logging(vn, "er:"+eright.ToString())

	return eleft.Equals(eright)
}

// converts a 2D bool slice to a single []byte, in which each bit represents a bool
// concatanates the table to one slice, then converts to bits using a helper function
// rows is included so that it can be converted back to a 2d slice
func tableToBits(table [][]bool) (int, []byte) {
	rows := len(table)

	flatslice := make([]bool, 0)
	for i := 0; i < rows; i++ {
		flatslice = append(flatslice, table[i]...)
	}

	bslice := booleansToBits(flatslice)

	return rows, bslice
}

// foil to tableToBits; converts the []byte of bits back into a table
func bitsToTable(rows int, bits []byte) [][]bool {
	bools := bitsToBooleans(bits)
	total := len(bools)
	var table [][]bool
	for i := 0; i < rows; i++ {
		temp := make([]bool, 0)
		for j := 0; j < (total / rows); j++ {
			temp = append(temp, bools[i*rows+j])
		}
		table = append(table, temp)
	}
	return table
}

// helper function for tableToBits
func booleansToBits(bools []bool) []byte {
	numBools := len(bools)
	numBits := (numBools + 7) / 8 // Calculate the number of bytes needed

	bits := make([]byte, numBits+1) // Add 1 extra byte to store the number of booleans
	bits[0] = byte(numBools)

	for i := 0; i < numBools; i++ {
		if bools[i] {
			bits[1+i/8] |= 1 << (7 - (i % 8))
		}
	}

	return bits
}

// helper function for bitsToTable
func bitsToBooleans(bits []byte) []bool {
	numBools := int(bits[0])
	bools := make([]bool, numBools)

	for i := 0; i < numBools; i++ {
		byteIndex := 1 + i/8
		bitIndex := 7 - (i % 8)
		bools[i] = (bits[byteIndex] & (1 << bitIndex)) != 0
	}

	return bools
}

// unused alternative to tableToBits
// BoolSliceToByteSlice converts a 2D bool slice to a byte slice.
func BoolSliceToByteSlice(boolSlice [][]bool) []byte {
	rows := len(boolSlice)
	cols := len(boolSlice[0])

	// Calculate the number of bytes needed for the boolean data.
	numBytes := (rows * cols) / 8
	if (rows*cols)%8 != 0 {
		numBytes++
	}

	// Create a byte slice to store the result.
	byteSlice := make([]byte, numBytes+2) // Extra 2 bytes for dimensions.

	// Store the dimensions in the first two bytes.
	byteSlice[0] = byte(rows)
	byteSlice[1] = byte(cols)

	// Flatten the 2D bool slice into the byte slice.
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			byteIndex := i*cols + j
			byteOffset := (byteIndex + 16) / 8 // Start from index 16 to avoid conflicting with dimensions.
			bitOffset := uint((byteIndex + 16) % 8)

			if boolSlice[i][j] {
				byteSlice[byteOffset] |= 1 << bitOffset
			}
		}
	}

	// Store the number of remainder bits in the first byte.
	byteSlice[0] |= byte((rows * cols) % 8)

	return byteSlice
}

// unused alternative to bitsToTable
// ByteSliceToBoolSlice converts a byte slice to a 2D bool slice.
func ByteSliceToBoolSlice(byteSlice []byte) [][]bool {
	// Extract dimensions from the first two bytes.
	rows := int(byteSlice[0])
	cols := int(byteSlice[1])

	// Calculate the number of bools in the 2D slice.
	numBools := rows * cols

	// Create a 2D bool slice to store the result.
	boolSlice := make([][]bool, rows)
	for i := range boolSlice {
		boolSlice[i] = make([]bool, cols)
	}

	// Extract bools from the byte slice and populate the 2D bool slice.
	for i := 0; i < numBools; i++ {
		byteIndex := (i + 16) / 8 // Start from index 16 to avoid conflicting with dimensions.
		bitOffset := uint((i + 16) % 8)

		boolSlice[i/cols][i%cols] = (byteSlice[byteIndex] & (1 << bitOffset)) != 0
	}

	return boolSlice
}

// checks a 2d byte slice for a byte slice, returns a bool, and the index if true
// originally intended to find a key, but can be used in any general case
func contains(keylist [][]byte, key []byte) (bool, int) {
	for i := 0; i < len(keylist); i++ {
		if bytes.Equal(key, keylist[i]) {
			return true, i
		}
	}
	return false, 0
}

// returns a list of keys from the keylist matching the indexes
func retrieveKeys(keylist [][]byte, indexes []int) []byte {
	temp := make([]byte, 0)
	for i := 0; i < len(indexes); i++ {
		temp = append(temp, keylist[indexes[i]]...)
	}
	return temp
}

// returns a list of names from the namelist matching the indexes
func retrieveNames(namelist []string, indexes []int) []string {
	temp := make([]string, 0)
	for i := 0; i < len(indexes); i++ {
		temp = append(temp, namelist[indexes[i]])
	}
	return temp
}

// Alternative VerifyAggregation function for testing, when VoteNode cannot be accessed
func TestVerifyAggregation(keylist [][]byte, ad AggregateBytes, checkXOR []byte) bool {
	sigmaagg := ad.Sigmaagg
	pubkeys := retrieveKeys(keylist, ad.Indexes)
	messages := ad.Messages
	table := bitsToTable(ad.Rows, ad.Table)

	eleft := BLS12383.NewFP12int(1)

	// checkHash := byteXOR(vn.msgTmp, vn.CurrentCID.Bytes())

	for i := 0; i < len(pubkeys)/97; i++ {
		currentrow := table[i]
		var currenthash []byte
		tempkey := pubkeys[i*97 : (i+1)*97]
		for j := 0; j < len(pubkeys)/97; j++ {
			if currentrow[j] {
				currenthash = append(currenthash, messages[j*36:(j+1)*36]...)
			}
		}
		currenthash = append(currenthash, tempkey...)

		cont := bytes.Contains(currenthash, checkXOR)
		if !cont {
			return false
		}

		pubkey := BLS12383.ECP2_fromBytes(pubkeys[i*97 : (i+1)*97])
		H := miracl.NewHASH256()
		H.Process_array(currenthash)
		Hm := BLS12383.BLS_hash_to_point(H.Hash())
		ei := BLS12383.Ate(pubkey, Hm)
		ei = BLS12383.Fexp(ei)
		eleft.Mul(ei)
	}
	sigP := BLS12383.ECP_fromBytes(sigmaagg)
	eright := BLS12383.Ate(BLS12383.ECP2_generator(), sigP)
	eright = BLS12383.Fexp(eright)

	return eleft.Equals(eright)
}

// converts a string into a []string, in which each string is of length x
func stringbreak(concatenated string, x int) []string {
	var strs []string

	// Split the concatenated string into substrings of length x
	for i := 0; i < len(concatenated); i += x {
		// Make sure not to go out of bounds when slicing
		end := i + x
		if end > len(concatenated) {
			end = len(concatenated)
		}
		// Append the substring to the slice
		strs = append(strs, concatenated[i:end])
	}

	// Return the slice
	return strs
}
