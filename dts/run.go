package dts

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"

	"time"
)

// var loopctx context.Context
// var joininterval int64 = 3

// var p = 1
// var mesh = false

// var name = 0

// main sets up the listener for the exit command, creates a votenode and starts the voting process
func Run(name int, mesh bool, testnode bool, address string) {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Wait for a signal
		<-sigCh
		fmt.Println("Received interrupt. Cleaning up and exiting gracefully.")
		// Perform cleanup tasks here

		// Exit the program
		os.Exit(0)
	}()

	flag.Parse()
	fmt.Println("startTest ", name)
	fmt.Println("address: " + address)
	retipfs, retnode := StartNode(mesh, false)
	vn := NewVoteNode(name, retipfs, retnode, address)

	ct, cancel := context.WithCancel(context.Background())
	defer cancel()
	fmt.Println(ct)
	Loopctx = ct
	periodicLoop(vn, ct, name, testnode)
	fmt.Println("after finishing all functions")

	fmt.Println("end of main")

}

func doPeriodically(vn *VoteNode, t time.Time, i int, tn bool) {
	fmt.Println("time to judge join or not")
	rand.Seed(time.Now().UnixNano())
	if rand.Float64() <= P {
		res := vn.Vote(t, i, tn)
		AddBin(res, vn)
	}
}

// periodicLoop sets up the loop
func periodicLoop(vn *VoteNode, loopctx context.Context, i int, tn bool) {
	ticker := time.NewTicker(time.Duration(Joininterval) * time.Second)
	// defer ticker.Stop()
	// fmt.Println("start watcher : ", ticker)

	// 性能測定用 3秒ごとにiが増加し、７回ごとに署名を実施する。
	// まずは署名希望回数7を設定20回署名で420秒
	for i := 0; true; i++ {
		select {
		case <-loopctx.Done():
			fmt.Println("Done")
			return
		case t := <-ticker.C:
			doPeriodically(vn, t, i, tn)
			fmt.Println("Tick at", i, t)
		}
	}
}
