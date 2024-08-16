package dts

// package main

import (
	// "encoding/hex"

	"math/rand"
	miracl "miracl/core"
	"miracl/core/BLS12383"
)

// "github.com/ipfs/go-cid"

func sign(message []byte, secretkey BLS12383.BIG) *BLS12383.ECP {
	H := miracl.NewHASH256()
	H.Process_array(message)
	// bls_hash_to_pointがexportedでなかったため名前を変更した
	G := BLS12383.BLS_hash_to_point(H.Hash())
	sG := BLS12383.G1mul(G, &secretkey)
	// b := make([]byte, 96)
	// sG.ToBytes(b, true)

	return sG //hex.EncodeToString(b)
}

func verify(sig *BLS12383.ECP, publickkeyg2 *BLS12383.ECP2, message []byte) bool {
	// sig_decoded, err := hex.DecodeString(sig)
	// if err != nil {
	// 	panic(err)
	// }
	// publickkeyg2_decoded, err := hex.DecodeString(publickkeyg2)
	// if err != nil {
	// 	panic(err)
	// }
	// sigP := BLS12383.ECP_fromBytes(sig_decoded)
	//signerg2 := BLS12383.ECP2_fromBytes(publickkeyg2_decoded)
	H := miracl.NewHASH256()
	H.Process_array([]byte(message))
	Hm := BLS12383.BLS_hash_to_point(H.Hash())
	Q := BLS12383.ECP2_generator()
	sigq := BLS12383.Ate(Q, sig)
	sigq = BLS12383.Fexp(sigq)
	hmpk := BLS12383.Ate(publickkeyg2, Hm)
	hmpk = BLS12383.Fexp(hmpk)

	return sigq.Equals(hmpk)
}

func genKey() (*BLS12383.ECP, *BLS12383.ECP2, *BLS12383.BIG) {
	RAW := make([]byte, 100)
	rng := miracl.NewRAND()
	rng.Clean()
	for i := 0; i < 100; i++ {
		RAW[i] = byte(rand.Intn(256))
	}
	rng.Seed(100, RAW)

	G := BLS12383.ECP_generator()
	Q := BLS12383.ECP2_generator()
	r := BLS12383.NewBIGints(BLS12383.CURVE_Order)
	secretKey := BLS12383.Randtrunc(r, 16*BLS12383.AESKEY, rng)
	publicKeyG1 := BLS12383.G1mul(G, secretKey)
	publicKeyG2 := BLS12383.G2mul(Q, secretKey)

	return publicKeyG1, publicKeyG2, secretKey
}
