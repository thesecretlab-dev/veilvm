package main

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ava-labs/libevm/crypto"
)

func TestVerifyWalletSigAnvil(t *testing.T) {
	pk, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ToLower(crypto.PubkeyToAddress(pk.PublicKey).Hex())
	if want != "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266" {
		t.Fatalf("unexpected anvil addr %s", want)
	}
	nonce := "0x" + strings.Repeat("ab", 32)
	msg := nativeOrderMessage("bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL", "market1", "buy", "yes", want, nonce, 25)
	sig, err := crypto.Sign(personalSignHash(msg), pk)
	if err != nil {
		t.Fatal(err)
	}
	sig[64] += 27 // MetaMask-style v
	sigHex := "0x" + hex.EncodeToString(sig)
	if err := verifyWalletSig(want, msg, sigHex); err != nil {
		t.Fatal(err)
	}
	if err := verifyWalletSig("0x0000000000000000000000000000000000000001", msg, sigHex); err == nil {
		t.Fatal("wrong wallet must fail")
	}
}
