#!/usr/bin/env node
/**
 * Persist v1 companion rails on local anvil (chainId 31337).
 * Teleporter is LocalTeleporter (mock). Not Fuji ICTT.
 */
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const here = dirname(fileURLToPath(import.meta.url));
const contractsDir = resolve(here, "../../veil-contracts");
const outFile = resolve(here, "companion-evm.addresses.json");
const EVM_RPC = process.env.EVM_RPC_URL || "http://127.0.0.1:8545";
const PK =
  process.env.EVM_RELAY_EXECUTOR_PRIVATE_KEY ||
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const CAST = process.env.CAST_BIN || "cast";
const FORGE = process.env.FORGE_BIN || "forge";
const FOUNDATION = { FOUNDRY_PROFILE: "rails", ...process.env };

function run(bin, args, opts = {}) {
  const r = spawnSync(bin, args, {
    encoding: "utf8",
    cwd: opts.cwd || contractsDir,
    env: { ...FOUNDATION, ...(opts.env || {}) },
  });
  if (r.status !== 0) {
    throw new Error(`${bin} ${args.join(" ")}\n${r.stderr || r.stdout}`);
  }
  return (r.stdout || "").trim();
}

async function rpc(method, params) {
  const res = await fetch(EVM_RPC, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
  });
  const json = await res.json();
  if (json.error) throw new Error(`${method}: ${json.error.message}`);
  return json.result;
}

async function hasCode(addr) {
  if (!addr || !String(addr).startsWith("0x")) return false;
  const code = await rpc("eth_getCode", [addr, "latest"]);
  return Boolean(code && code !== "0x" && code !== "0x0");
}

function parseDeployed(out) {
  const m = out.match(/Deployed to:\s+(0x[a-fA-F0-9]{40})/);
  if (!m) throw new Error(`deploy parse failed:\n${out}`);
  return m[1];
}

function deploy(contract, ctor = []) {
  const args = ["create", contract, "--rpc-url", EVM_RPC, "--private-key", PK, "--broadcast"];
  if (ctor.length) args.push("--constructor-args", ...ctor);
  return parseDeployed(run(FORGE, args));
}

const chainHex = await rpc("eth_chainId", []);
const chainId = parseInt(chainHex, 16);
if (chainId !== 31337) {
  throw new Error(`expected anvil 31337, got ${chainId}`);
}

const owner = run(CAST, ["wallet", "address", PK], { cwd: here });
let prev = {};
if (existsSync(outFile)) {
  prev = JSON.parse(readFileSync(outFile, "utf8"));
}

async function ensure(name, current, fn) {
  if (await hasCode(current)) {
    console.log(`keep ${name} ${current}`);
    return current;
  }
  const addr = fn();
  console.log(`deploy ${name} ${addr}`);
  return addr;
}

const wveil = await ensure("wveil", prev.wveil, () =>
  deploy("contracts/core/WVEIL.sol:WVEIL"),
);
const faucet = await ensure("faucet", prev.faucet, () =>
  deploy("contracts/core/VeilFaucet.sol:VeilFaucet"),
);
const orderGw = await ensure("orderIntentGateway", prev.orderIntentGateway, () =>
  deploy("contracts/bridge/VeilOrderIntentGateway.sol:VeilOrderIntentGateway", [owner, owner]),
);
const liqGw = await ensure("liquidityIntentGateway", prev.liquidityIntentGateway, () =>
  deploy("contracts/bridge/VeilLiquidityIntentGateway.sol:VeilLiquidityIntentGateway", [owner, owner]),
);
const teleporter = await ensure("teleporterMessenger", prev.teleporterMessenger, () =>
  deploy("contracts/bridge/LocalTeleporter.sol:LocalTeleporter"),
);

const veilChainId = run(CAST, ["keccak", "local-veilvm"], { cwd: here });
const bridge = await ensure("bridgeMinterContract", prev.bridgeMinterContract, () =>
  deploy("contracts/bridge/VeilBridgeMinter.sol:VeilBridgeMinter", [
    wveil,
    teleporter,
    owner,
    veilChainId,
    "1000000000000000000000",
    "3600",
  ]),
);

const doc = {
  network: "local-anvil",
  chainId: 31337,
  rpcUrl: EVM_RPC,
  status: "local-rails-deployed",
  abandoned: false,
  teleporterKind: "local-mock",
  create2Deployer: "0x4e59b44847b379578588920cA78FbF26c0B4956C",
  tempAdminEoa: owner,
  deployer1: owner,
  bridgeRelayer1: owner,
  bridgeRelayer2: owner,
  opsKeeper1: owner,
  treasuryFundingAddress: "",
  teleporterRegistry: teleporter,
  teleporterMessenger: teleporter,
  nativeMinter: "",
  txAllowList: "",
  contractDeployerAllowList: "",
  wveil,
  bridgeMinterContract: bridge,
  orderIntentGateway: orderGw,
  liquidityIntentGateway: liqGw,
  feeConfigManager: "",
  multicall3: "",
  faucet,
  deployedBy: owner,
  deployedAt: new Date().toISOString(),
  notes:
    "Local anvil 31337 rails. Teleporter is LocalTeleporter (mock), not Fuji ICTT. Precompiles empty on purpose. Do not use as Fuji/mainnet registry.",
};

writeFileSync(outFile, JSON.stringify(doc, null, 2) + "\n");
console.log(`wrote ${outFile}`);
console.log(JSON.stringify({ wveil, faucet, orderGw, liqGw, teleporter, bridge }, null, 2));
