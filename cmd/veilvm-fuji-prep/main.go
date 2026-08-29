// Fuji genesis prep: derive HyperSDK addresses we control and write genesis-fuji.json.
// Never prints private keys.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
	vgenesis "github.com/ava-labs/hypersdk/examples/veilvm/genesis"
)

const (
	keyboxPrivate = `C:\Users\Justin\tools\veil-keybox\private`
	fujiNetworkID = 5
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readHexFile(path string) []byte {
	raw, err := os.ReadFile(path)
	must(err)
	s := strings.TrimSpace(string(raw))
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	b, err := hex.DecodeString(s)
	must(err)
	return b
}

func writePrivateHex(path string, b []byte) {
	must(os.WriteFile(path, []byte(hex.EncodeToString(b)+"\n"), 0o600))
}

func ed25519FromFile(path string) ed25519.PrivateKey {
	b := readHexFile(path)
	if len(b) != ed25519.PrivateKeyLen {
		must(fmt.Errorf("%s: got %d bytes, want %d", path, len(b), ed25519.PrivateKeyLen))
	}
	var pk ed25519.PrivateKey
	copy(pk[:], b)
	return pk
}

func ensureTreasury() (ed25519.PrivateKey, bool) {
	path := filepath.Join(keyboxPrivate, "genesis-treasury.hex")
	if _, err := os.Stat(path); err == nil {
		return ed25519FromFile(path), false
	}
	pk, err := ed25519.GeneratePrivateKey()
	must(err)
	writePrivateHex(path, pk[:])
	return pk, true
}

func main() {
	root := filepath.Clean(filepath.Join("."))
	if filepath.Base(root) != "veilvm" {
		// allow running from repo root or cmd dir
		if _, err := os.Stat("genesis.json"); err != nil {
			must(os.Chdir(`C:\Users\Justin\src\veil\veilvm`))
		}
	}

	raw, err := os.ReadFile("genesis.json")
	must(err)
	g := &vgenesis.Genesis{}
	must(json.Unmarshal(raw, g))
	if g.Tokenomics == nil {
		must(fmt.Errorf("genesis.json missing tokenomics"))
	}

	proverPath := filepath.Join(keyboxPrivate, "prover-authority.hex")
	prover := ed25519FromFile(proverPath)
	treasury, created := ensureTreasury()

	treasuryAddr := auth.NewED25519Address(treasury.PublicKey())
	proverAddr := auth.NewED25519Address(prover.PublicKey())

	if len(g.CustomAllocation) == 0 {
		must(fmt.Errorf("genesis.json missing customAllocation"))
	}
	g.CustomAllocation[0].Address = treasuryAddr
	g.Tokenomics.Governance = treasuryAddr
	g.Tokenomics.Operations = treasuryAddr
	g.Tokenomics.MintAuthority = treasuryAddr
	g.Tokenomics.ProverAuthority = proverAddr
	if g.Rules != nil {
		g.Rules.NetworkID = fujiNetworkID
		g.Rules.ChainID = ids.Empty
	}

	out, err := json.MarshalIndent(g, "", "  ")
	must(err)
	must(os.WriteFile("genesis-fuji.json", append(out, '\n'), 0o644))

	loaded, _, err := vgenesis.Factory{}.Load(out, nil, fujiNetworkID, ids.Empty)
	must(err)
	gg := loaded.(*vgenesis.Genesis)
	if gg.Tokenomics.TotalSupply != 990_999_000 {
		must(fmt.Errorf("totalSupply %d", gg.Tokenomics.TotalSupply))
	}

	evidDir := filepath.Join("evidence-bundles", "fuji-l1")
	must(os.MkdirAll(evidDir, 0o755))

	vmID := mconsts.ID
	vmIDPath := filepath.Join(evidDir, "vm-id.txt")
	vmBody := fmt.Sprintf(`vmID=%s
derivedFrom=name:veilvm
method=consts.ID = ids.ToID(bytes("veilvm") padded to 32)
recordedAt=%s
gitHEAD=11aa8e3
canonicalTag=veilvm-canonical-2026-08-24
canonicalTagSHA=05943c7f02e41304053f76fe642986cc1019745a
localPluginSHA256=F5C990DA3C6A17D2A11A2DC739EB512285331455DF2432D11AA1EF887A27606A
localAvalanchego=1.13.0 rpcchainvm=39
fujiPublic=avalanchego/1.15.0 rpcProtocolVersion=46
note=VM ID is from the name veilvm, not git SHA. Fuji node needs plugin built for rpcchainvm 46.
`, vmID.String(), time.Now().UTC().Format(time.RFC3339))
	must(os.WriteFile(vmIDPath, []byte(vmBody), 0o644))

	addrs := map[string]any{
		"network":           "fuji",
		"networkID":         fujiNetworkID,
		"vmID":              vmID.String(),
		"genesisFile":       "genesis-fuji.json",
		"genesisTreasury":   treasuryAddr.String(),
		"proverAuthority":   proverAddr.String(),
		"treasuryCreated":   created,
		"chainIDJSON":       ids.Empty.String(),
		"chainIDJSONNote":   "ids.Empty placeholder; Factory.Load overwrites from avalanchego chain context. Same CB58 as Primary Network ID. Not a claim that VEIL is Primary.",
		"doNotUse":          "0xB9a05AFC8eff7eE6a84889Bb9C88A89eAA2f96af",
		"localGenesisLeft":  "genesis.json unchanged (local stack)",
		"lostOwner":         "do-not-deploy-under",
	}
	ab, err := json.MarshalIndent(addrs, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(evidDir, "addresses.json"), append(ab, '\n'), 0o644))

	pubIndex := filepath.Join(`C:\Users\Justin\tools\veil-keybox`, "index.public.json")
	fmt.Printf("vmID=%s\n", vmID.String())
	fmt.Printf("treasury=%s created=%v\n", treasuryAddr, created)
	fmt.Printf("prover=%s\n", proverAddr)
	fmt.Printf("wrote genesis-fuji.json %s %s\n", vmIDPath, filepath.Join(evidDir, "addresses.json"))
	_ = pubIndex
}
