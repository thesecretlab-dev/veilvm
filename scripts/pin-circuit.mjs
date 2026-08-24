#!/usr/bin/env node
/**
 * D05: pin shielded-ledger-v1 fixture hashes. Does not reuse Feb evidence.
 */
import { createHash } from "crypto";
import { readFileSync, mkdirSync, writeFileSync } from "fs";
import { dirname, resolve, relative } from "path";
import { fileURLToPath } from "url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const fixture = resolve(root, "zk-fixture-new");

const files = [
  "groth16_shielded_ledger_vk.bin",
  "groth16_shielded_ledger_pk.bin",
  "sample_shielded_ledger_preimage.hex",
  "sample_shielded_ledger_proof.bin",
  "sample_shielded_ledger_proof_envelope.bin",
  "sample_shielded_ledger_public_inputs_hash.hex",
  "sample_shielded_ledger_public_witness.bin",
];

function sha256File(p) {
  return createHash("sha256").update(readFileSync(p)).digest("hex");
}

const hashes = {};
for (const name of files) {
  hashes[name] = sha256File(resolve(fixture, name));
}

const report = {
  id: "D05",
  timestamp: new Date().toISOString(),
  circuitID: "shielded-ledger-v1",
  proofType: "groth16",
  curve: "bn254",
  fixtureDir: "zk-fixture-new",
  hashes,
  notes: [
    "Pinned from this tree, not Feb launch-gate evidence.",
    "clearhash-v1 fixtures remain on disk for archaeology and are not the production circuit.",
    "Verifier requiredCircuitID must be shielded-ledger-v1.",
  ],
  pass: true,
};

const outDir = resolve(root, "evidence-bundles/zk-circuit-assurance");
mkdirSync(outDir, { recursive: true });
const stamp = new Date().toISOString().replace(/[-:T.Z]/g, "").slice(0, 14);
const outName = `zk-circuit-assurance-${stamp}.json`;
writeFileSync(resolve(outDir, outName), JSON.stringify(report, null, 2) + "\n");
writeFileSync(resolve(outDir, "latest.txt"), outName + "\n");
console.log(JSON.stringify(report, null, 2));
console.error(`wrote ${relative(root, resolve(outDir, outName))}`);
