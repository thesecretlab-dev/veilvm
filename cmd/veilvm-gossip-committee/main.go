package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/curve25519"
)

func main() {
	n := flag.Int("n", 3, "committee size")
	out := flag.String("out", ".", "output directory")
	flag.Parse()
	if *n < 2 {
		fatalf("n must be >= 2")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}
	pubs := make([]string, 0, *n)
	var node0priv string
	for i := 0; i < *n; i++ {
		var priv [32]byte
		if _, err := rand.Read(priv[:]); err != nil {
			fatalf("rand: %v", err)
		}
		pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
		if err != nil {
			fatalf("x25519: %v", err)
		}
		privHex := hex.EncodeToString(priv[:])
		pubHex := hex.EncodeToString(pub)
		if err := os.WriteFile(filepath.Join(*out, fmt.Sprintf("node%d.priv", i)), []byte(privHex+"\n"), 0o600); err != nil {
			fatalf("write priv: %v", err)
		}
		if err := os.WriteFile(filepath.Join(*out, fmt.Sprintf("node%d.pub", i)), []byte(pubHex+"\n"), 0o644); err != nil {
			fatalf("write pub: %v", err)
		}
		pubs = append(pubs, pubHex)
		if i == 0 {
			node0priv = privHex
		}
	}
	committee := strings.Join(pubs, ",")
	if err := os.WriteFile(filepath.Join(*out, "committee.csv"), []byte(committee+"\n"), 0o644); err != nil {
		fatalf("write committee: %v", err)
	}
	fmt.Printf("NODE_PRIV=%s\n", node0priv)
	fmt.Printf("COMMITTEE=%s\n", committee)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
