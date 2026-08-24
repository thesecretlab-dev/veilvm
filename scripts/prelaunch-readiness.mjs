#!/usr/bin/env node
/**
 * Local-profile prelaunch (C06).
 * Does not authorize Fuji or mainnet. productionLaunchPass is always false here.
 */
import { existsSync, readFileSync, mkdirSync, writeFileSync } from "fs";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const stamp = new Date().toISOString();
const fileStamp = stamp.replace(/[-:T.Z]/g, "").slice(0, 14);

const NODE = process.env.NODE_URL || "http://127.0.0.1:9660";
const ROUTER = process.env.ORDER_ROUTER_URL || "http://127.0.0.1:9098";
const EVM = process.env.EVM_RPC_URL || "http://127.0.0.1:8545";

function check(id, ok, detail) {
  return { id, ok: Boolean(ok), detail: detail || "" };
}

async function getJson(url, timeoutMs = 4000) {
  const ac = new AbortController();
  const t = setTimeout(() => ac.abort(), timeoutMs);
  try {
    const res = await fetch(url, { signal: ac.signal });
    const text = await res.text();
    let json = null;
    try {
      json = JSON.parse(text);
    } catch {
      json = { raw: text.slice(0, 200) };
    }
    return { ok: res.ok, status: res.status, json };
  } finally {
    clearTimeout(t);
  }
}

async function postRpc(url, method, params = []) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  return res.json();
}

const chainMetaPath = resolve(root, ".local/local-chain.json");
const chainMeta = existsSync(chainMetaPath)
  ? JSON.parse(readFileSync(chainMetaPath, "utf8"))
  : {};
const chainID = chainMeta.chainID || "";
const addressesPath = resolve(here, "companion-evm.addresses.json");
const addresses = existsSync(addressesPath)
  ? JSON.parse(readFileSync(addressesPath, "utf8"))
  : {};
const genesis = JSON.parse(readFileSync(resolve(root, "genesis.json"), "utf8"));
const zkCfgPath = chainID
  ? resolve(root, `.local/nodedata/configs/chains/${chainID}/config.json`)
  : "";
const zkCfg = zkCfgPath && existsSync(zkCfgPath)
  ? JSON.parse(readFileSync(zkCfgPath, "utf8"))
  : null;

const health = await getJson(`${NODE}/ext/health`);
const ready = await getJson(`${NODE}/ext/health/readiness`);
let height = null;
if (chainID) {
  try {
    const wr = await postRpc(`${NODE}/ext/bc/${chainID}/coreapi`, "coreapi.LastAccepted");
    height = wr?.result ?? wr;
  } catch (err) {
    height = { error: String(err) };
  }
}

const router = await getJson(`${ROUTER}/health`);
let evmChainId = null;
try {
  const evm = await postRpc(EVM, "eth_chainId");
  evmChainId = parseInt(evm.result, 16);
} catch (err) {
  evmChainId = null;
}

const zk = zkCfg?.controller?.zk || {};
const abandonmentNote = existsSync(resolve(root, "evidence-bundles/abandoned-feb-2026/README.md"));
const abandonedPacket = existsSync(
  resolve(root, "evidence-bundles/abandoned-feb-2026/companion-evm.addresses.json"),
);

const checks = [
  check(
    "c01Health",
    health.json?.healthy === true,
    `healthy=${health.json?.healthy} veilHeight=${health.json?.checks?.[chainID]?.message?.engine?.consensus?.lastAcceptedHeight}`,
  ),
  check("c01Readiness", ready.ok && (ready.json?.healthy === true || ready.status === 200), `ready status=${ready.status}`),
  check(
    "c02ZkVerifier",
    zk.enabled === true && zk.strict === true && zk.requiredCircuitID === "shielded-ledger-v1" && Boolean(zk.groth16VerifyingKeyPath),
    JSON.stringify(zk),
  ),
  check("c03ChainProducing", Boolean(chainID), `chainID=${chainID}`),
  check("c04CompanionAnvil", evmChainId === 31337, `eth_chainId=${evmChainId}`),
  check("c04OrderRouter", router.json?.ok === true, JSON.stringify(router.json)),
  check(
    "c05AbandonedFebCompanion",
    abandonmentNote && abandonedPacket && Number(addresses.chainId) !== 22207 && addresses.abandoned !== true,
    `live chainId=${addresses.chainId} abandonedFlag=${addresses.abandoned}`,
  ),
  check("supplyFreeze", Number(genesis?.tokenomics?.totalSupply) === 990999000, `tokenomics.totalSupply=${genesis?.tokenomics?.totalSupply}`),
];

const localRequired = ["c01Health", "c01Readiness", "c02ZkVerifier", "c04CompanionAnvil", "c04OrderRouter", "c05AbandonedFebCompanion", "supplyFreeze"];
const localPass = localRequired.every((id) => checks.find((c) => c.id === id)?.ok);

const report = {
  profile: "local-windows",
  network: "local",
  timestamp: new Date().toISOString(),
  stamp,
  overallPass: localPass,
  productionLaunchPass: false,
  node: NODE,
  chainID,
  subnetID: chainMeta.subnetID || "",
  nodeID: chainMeta.nodeID || "",
  companion: { rpc: EVM, chainId: evmChainId },
  router: ROUTER,
  checks: Object.fromEntries(
    checks.map((c) => [c.id, { ok: c.ok, detail: c.detail }]),
  ),
  productionGates: {
    checklist: { ok: false, reason: "Feb production checklist is STALE-2026-02" },
    companionPrimitives: { ok: false, reason: "Teleporter/bridge not deployed on local anvil (Phase F)" },
    tokenomics: { ok: false, reason: "D01 launchpad freeze not run" },
    auditClosure: { ok: false, reason: "not a production profile" },
    flywheelAudit: { ok: false, reason: "not a production profile" },
    economicCoherence: { ok: false, reason: "not a production profile" },
    animaReadiness: { ok: false, reason: "Fuji RPC not pointed; Phase E/G" },
  },
  evidence: [
    "evidence-bundles/local-revive-2026-08-24/smoke.md",
    "evidence-bundles/local-revive-2026-08-24/stack-e2e.md",
    "evidence-bundles/abandoned-feb-2026/README.md",
  ],
};

const outDir = resolve(root, "evidence-bundles/control-tower");
mkdirSync(outDir, { recursive: true });
const outFile = resolve(outDir, `prelaunch-readiness-${fileStamp}.json`);
writeFileSync(outFile, JSON.stringify(report, null, 2) + "\n");
writeFileSync(resolve(outDir, "latest-local.txt"), `prelaunch-readiness-${fileStamp}.json\n`);

console.log(JSON.stringify(report, null, 2));
console.error(`wrote ${outFile}`);
process.exitCode = localPass ? 0 : 1;
