package zk

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mconsts "github.com/ava-labs/hypersdk/examples/veilvm/consts"
)

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "zk-fixture-new", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("missing fixture %s: %v", p, err)
	}
	return p
}

func TestPinnedShieldedLedgerVKLoads(t *testing.T) {
	v, err := NewVerifier(Config{
		Groth16VerifyingKeyPath: fixturePath(t, "groth16_shielded_ledger_vk.bin"),
		RequiredCircuitID:       mconsts.ProofCircuitShieldedLedgerV1,
	})
	if err != nil {
		t.Fatalf("load pinned vk: %v", err)
	}
	if v.requiredCircuitID != mconsts.ProofCircuitShieldedLedgerV1 {
		t.Fatalf("required circuit %q", v.requiredCircuitID)
	}

	proof, err := os.ReadFile(fixturePath(t, "sample_shielded_ledger_proof.bin"))
	if err != nil {
		t.Fatal(err)
	}
	hashHex, err := os.ReadFile(fixturePath(t, "sample_shielded_ledger_public_inputs_hash.hex"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := hex.DecodeString(strings.TrimSpace(string(hashHex)))
	if err != nil {
		t.Fatal(err)
	}
	wit, err := os.ReadFile(fixturePath(t, "sample_shielded_ledger_public_witness.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(mconsts.ProofTypeGroth16, mconsts.ProofCircuitShieldedLedgerV1, proof, hash, wit); err != nil {
		t.Fatalf("pinned sample must verify against pinned vk: %v", err)
	}
	if err := v.Verify(mconsts.ProofTypeGroth16, mconsts.ProofCircuitClearHashV1, proof, hash, wit); err == nil {
		t.Fatalf("clearhash-v1 must not verify under shielded-ledger-v1 required circuit")
	}
}

func TestPinnedVKHashStable(t *testing.T) {
	raw, err := os.ReadFile(fixturePath(t, "groth16_shielded_ledger_vk.bin"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	const want = "40d25f181550c879f93d22dfa50305700bdb0e731ced46d1b789248e552398ba"
	got := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("pinned vk hash changed: got=%s want=%s", got, want)
	}
}
