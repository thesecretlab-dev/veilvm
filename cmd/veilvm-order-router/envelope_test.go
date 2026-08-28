package main

import (
	"bytes"
	"testing"
)

func TestSealEnvelopeIsOpaque(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plain := []byte("veil-order-v1|market|buy|yes|25.00000000|0xf39|nonce")
	env, err := sealEnvelope(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !isSealedEnvelope(env) {
		t.Fatal("missing VEILENC1 magic")
	}
	if envelopeLooksPlaintext(env) {
		t.Fatalf("ciphertext leaked plaintext: %q", env)
	}
	got, err := openEnvelope(key, env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("roundtrip mismatch: %q", got)
	}
	if _, err := openEnvelope(bytes.Repeat([]byte{8}, 32), env); err == nil {
		t.Fatal("wrong key must fail")
	}
}
