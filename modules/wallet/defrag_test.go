package wallet

import (
	"testing"
	"time"

	"github.com/NebulousLabs/Sia/types"
)

// TestDefragWallet mines many blocks and checks that the wallet's outputs are
// consolidated once more than defragThreshold blocks are mined.
func TestDefragWallet(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	wt, err := createWalletTester("TestDefragWallet")
	if err != nil {
		t.Fatal(err)
	}
	defer wt.closeWt()

	// mine defragThreshold blocks, resulting in defragThreshold outputs
	for i := 0; i < defragThreshold; i++ {
		_, err := wt.miner.AddBlock()
		if err != nil {
			t.Fatal(err)
		}
	}

	// add another block to push the number of outputs over the threshold
	_, err = wt.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}

	// allow some time for the defrag transaction to occur, then mine another block
	time.Sleep(time.Second)

	_, err = wt.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}

	// defrag should keep the outputs below the threshold
	if len(wt.wallet.siacoinOutputs) > defragThreshold {
		t.Fatalf("defrag should result in fewer than defragThreshold outputs, got %v wanted %v\n", len(wt.wallet.siacoinOutputs), defragThreshold)
	}
}

// TestDefragWalletDust verifies that dust outputs do not trigger the defrag
// operation.
func TestDefragWalletDust(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	wt, err := createWalletTester("TestDefragWalletDust")
	if err != nil {
		t.Fatal(err)
	}
	defer wt.closeWt()

	dustOutputValue := types.NewCurrency64(10000)
	noutputs := defragThreshold + 1

	tbuilder := wt.wallet.StartTransaction()
	err = tbuilder.FundSiacoins(dustOutputValue.Mul64(uint64(noutputs)))
	if err != nil {
		t.Fatal(err)
	}

	var dest types.UnlockHash
	for k := range wt.wallet.keys {
		dest = k
		break
	}

	for i := 0; i < noutputs; i++ {
		tbuilder.AddSiacoinOutput(types.SiacoinOutput{
			Value:      dustOutputValue,
			UnlockHash: dest,
		})
	}

	txns, err := tbuilder.Sign(true)
	if err != nil {
		t.Fatal(err)
	}

	err = wt.tpool.AcceptTransactionSet(txns)
	if err != nil {
		t.Fatal(err)
	}

	_, err = wt.miner.AddBlock()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	if len(wt.wallet.siacoinOutputs) < defragThreshold {
		t.Fatal("defrag consolidated dust outputs")
	}
}

// TestDefragOutputExhaustion verifies that a malicious actor cannot use the
// defragger to prevent a wallet from sending coins.
func TestDefragOutputExhaustion(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	wt, err := createWalletTester("TestDefragOutputExhaustion")
	if err != nil {
		t.Fatal(err)
	}
	defer wt.closeWt()

	// mine a bunch of blocks continuously at a high enough rate to keep the
	// defragger running.
	closechan := make(chan struct{})
	go func() {
		for {
			select {
			case <-closechan:
				return
			case <-time.After(time.Millisecond * 500):
				_, err := wt.miner.AddBlock()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}()

	time.Sleep(time.Second * 5)

	sendAmount := types.SiacoinPrecision.Mul64(2000)
	_, err = wt.wallet.SendSiacoins(sendAmount, types.UnlockHash{})
	if err != nil {
		t.Fatal(err)
	}

	close(closechan)
}
