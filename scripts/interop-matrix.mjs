#!/usr/bin/env node
/**
 * Fail-closed local interop matrix. Native VeilVM + companion anvil 31337 + router + frontend.
 * Local ≠ Fuji ≠ mainnet. App-id 22207 is not an EVM chain id.
 */
import { createHash, randomBytes } from "node:crypto";
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const here = dirname(fileURLToPath(import.meta.url));
const ROUTER = process.env.ORDER_ROUTER_URL || "http://127.0.0.1:9098";
const SECRET = process.env.ORDER_ROUTER_RELAY_SECRET || "local-dev-secret";
const FRONT = process.env.VEIL_FRONTEND || "http://127.0.0.1:3000";
const EVM = process.env.EVM_RPC_URL || "http://127.0.0.1:8545";
const CAST = process.env.CAST_BIN || "C:\\Users\\Justin\\tools\\foundry\\cast.exe";
const PK =
  process.env.EVM_RELAY_EXECUTOR_PRIVATE_KEY ||
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const CHAIN = "bdRGUMA7rzZFXjbn1ePTjqhAUfTjW94e69p7qZd4puZ3uEosL";

const failed = [];
function pass(id, detail) {
  console.log(`PASS  ${id}  ${detail}`);
}
function fail(id, detail) {
  failed.push(id);
  console.log(`FAIL  ${id}  ${detail}`);
}

async function getJson(url) {
  const res = await fetch(url, { cache: "no-store" });
  const json = await res.json().catch(() => ({}));
  return { status: res.status, json };
}

async function post(path, body, extraHeaders = {}) {
  const res = await fetch(`${ROUTER}${path}`, {
    method: "POST",
    headers: { "content-type": "application/json", "x-relay-secret": SECRET, ...extraHeaders },
    body: JSON.stringify(body),
  });
  const json = await res.json().catch(() => ({}));
  return { status: res.status, json };
}

function envelope() {
  const env = randomBytes(128);
  return {
    envelope: "0x" + env.toString("hex"),
    commitment: "0x" + createHash("sha256").update(env).digest("hex"),
    nullifier: "0x" + randomBytes(32).toString("hex"),
  };
}

const railsPath = resolve(here, "companion-evm.addresses.json");
const rails = JSON.parse(readFileSync(railsPath, "utf8"));

const net = await getJson(`${FRONT}/api/network-status`);
if (net.json?.ok && net.json?.blockHeight > 0 && net.json?.veilvm?.chainId === CHAIN) {
  pass("frontend.network-status", `height=${net.json.blockHeight}`);
} else {
  fail("frontend.network-status", JSON.stringify(net.json).slice(0, 240));
}

const interop = await getJson(`${FRONT}/api/interop`);
if (interop.json?.ok) {
  pass("frontend.interop", (interop.json.checks || []).map((c) => c.id).join(","));
} else {
  fail("frontend.interop", `failed=${(interop.json?.failed || []).join(",")}`);
}

if (net.json?.companion?.chainId === 31337 && net.json?.companion?.ok) {
  pass("companion.anvil", "31337 gateways live");
} else {
  fail("companion.anvil", JSON.stringify(net.json?.companion || {}).slice(0, 200));
}

const orders = await getJson(`${FRONT}/api/orders`);
if (orders.json?.ok && orders.json?.chainId === CHAIN) {
  pass("frontend.orders", orders.json.chainId);
} else {
  fail("frontend.orders", JSON.stringify(orders.json).slice(0, 200));
}

const swap = await post("/intents/native/liquidity/execute", {
  ...envelope(),
  operation: "swap_exact_in",
  assetIn: 1,
  assetOut: 0,
  amountIn: 4,
  minAmountOut: 1,
});
if (swap.json?.accepted && swap.json?.veilTxHash) {
  pass("native.amm", swap.json.veilTxHash);
} else {
  fail("native.amm", JSON.stringify(swap.json).slice(0, 240));
}

const evmBare = await post("/evm/intents/execute", {
  ...envelope(),
  marketKey: "11111111111111111111111111111111LpoYY",
  marketType: "veil_native",
});
if (evmBare.status === 400 && String(evmBare.json?.error || "").includes("sourceTxHash")) {
  pass("evm.fail-closed", evmBare.json.error);
} else {
  fail("evm.fail-closed", `${evmBare.status} ${JSON.stringify(evmBare.json).slice(0, 200)}`);
}

const created = await post("/native/create-market", {
  question: `interop ${new Date().toISOString()}`,
  outcomes: 2,
  creatorBond: 1,
});
let marketId = created.json?.marketId;
if (created.json?.accepted && marketId) {
  pass("native.market", marketId);
} else {
  const listed = await fetch(`${ROUTER}/markets`).then((r) => r.json()).catch(() => ({}));
  marketId = listed?.markets?.[0]?.marketId;
  if (marketId) {
    pass("native.market", `reuse ${marketId} (${String(created.json?.error || "create skipped").slice(0, 80)})`);
  } else {
    fail("native.market", JSON.stringify(created.json).slice(0, 200));
  }
}
if (marketId) {
  const env = envelope();
  const mailboxPath = resolve(here, "intent-mailbox.json");
  const mailbox = existsSync(mailboxPath) ? JSON.parse(readFileSync(mailboxPath, "utf8")) : {};
  mailbox[env.commitment] = {
    envelope: env.envelope,
    marketKey: marketId,
    marketType: "veil_native",
    routingFeeBps: 0,
    windowId: 1,
  };
  writeFileSync(mailboxPath, JSON.stringify(mailbox, null, 2));
  const gw = rails.orderIntentGateway;
  const send = spawnSync(
    CAST,
    ["send", gw, "submitIntent(bytes32,bytes32)", env.commitment, env.nullifier, "--rpc-url", EVM, "--private-key", PK, "--json"],
    { encoding: "utf8" },
  );
  if (send.status !== 0) {
    fail("companion.submitIntent", send.stderr || send.stdout);
  } else {
    let receipt;
    try {
      receipt = JSON.parse(send.stdout);
    } catch {
      const hash = (send.stdout || "").match(/0x[a-fA-F0-9]{64}/)?.[0];
      receipt = hash
        ? JSON.parse(spawnSync(CAST, ["receipt", hash, "--rpc-url", EVM, "--json"], { encoding: "utf8" }).stdout)
        : {};
    }
    const log0 = (receipt.logs || []).find((l) => String(l.address).toLowerCase() === String(gw).toLowerCase());
    const intentId = log0?.topics?.[1];
    const sourceTxHash = receipt.transactionHash || receipt.hash;
    if (!intentId || !sourceTxHash) {
      fail("companion.submitIntent", "no IntentSubmitted log");
    } else {
      pass("companion.submitIntent", sourceTxHash);
      const relayed = await post("/evm/intents/execute", {
        intentId,
        marketKey: marketId,
        envelope: env.envelope,
        commitment: env.commitment,
        nullifier: env.nullifier,
        marketType: "veil_native",
        routingFeeBps: 0,
        windowId: 1,
        sourceTxHash,
      });
      if (relayed.json?.accepted && relayed.json?.veilTxHash) {
        pass("evm.relay", relayed.json.veilTxHash);
      } else {
        fail("evm.relay", JSON.stringify(relayed.json).slice(0, 240));
      }
    }
  }
}

if (failed.length) {
  console.log(`INTEROP-MATRIX FAIL ${failed.join(",")}`);
  process.exit(1);
}
console.log("INTEROP-MATRIX PASS");
