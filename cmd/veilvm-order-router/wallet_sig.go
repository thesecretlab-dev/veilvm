package main

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ava-labs/libevm/crypto"
)

func nativeOrderMessage(chainID, marketID, side, outcome, wallet, nonce string, amount float64) string {
	return strings.Join([]string{
		"VEIL native order v1",
		"chain:" + chainID,
		"market:" + marketID,
		"side:" + strings.ToLower(strings.TrimSpace(side)),
		"outcome:" + strings.ToLower(strings.TrimSpace(outcome)),
		fmt.Sprintf("amountUsd:%.8f", amount),
		"wallet:" + strings.ToLower(strings.TrimSpace(wallet)),
		"nonce:" + strings.ToLower(strings.TrimSpace(nonce)),
	}, "\n")
}

func personalSignHash(message string) []byte {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	return crypto.Keccak256([]byte(prefix), []byte(message))
}

func decodeSigHex(sig string) ([]byte, error) {
	s := strings.TrimSpace(sig)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("walletSignature: %w", err)
	}
	if len(b) != 65 {
		return nil, fmt.Errorf("walletSignature must be 65 bytes, got %d", len(b))
	}
	if b[64] >= 27 {
		b[64] -= 27
	}
	if b[64] > 1 {
		return nil, fmt.Errorf("walletSignature recovery id %d", b[64])
	}
	return b, nil
}

func recoverPersonalAddress(message, sigHex string) (string, error) {
	sig, err := decodeSigHex(sigHex)
	if err != nil {
		return "", err
	}
	pub, err := crypto.SigToPub(personalSignHash(message), sig)
	if err != nil {
		return "", fmt.Errorf("walletSignature recover: %w", err)
	}
	return strings.ToLower(crypto.PubkeyToAddress(*pub).Hex()), nil
}

func verifyWalletSig(wantAddr, message, sigHex string) error {
	want := strings.ToLower(strings.TrimSpace(wantAddr))
	if !strings.HasPrefix(want, "0x") || len(want) != 42 {
		return fmt.Errorf("walletAddress must be a 20-byte 0x address")
	}
	got, err := recoverPersonalAddress(message, sigHex)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("walletSignature recovered %s, not %s", got, want)
	}
	return nil
}

func parseNonceHex(nonce string) ([]byte, error) {
	s := strings.TrimSpace(nonce)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("walletNonce: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("walletNonce must be 32 bytes, got %d", len(b))
	}
	return b, nil
}
