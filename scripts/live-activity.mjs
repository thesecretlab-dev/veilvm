#!/usr/bin/env node
/**
 * Keeps the local VeilVM chain feeling live: AMM ticks, fee routing,
 * occasional native orders + groth16 clears. Not Fuji. Detach from the agent job.
 */
import { createHash, randomBytes } from "node:crypto";
import { appendFileSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const here = dirname(fileURLToPath(import.meta.url));
const logDir = resolve(here, "..", ".local", "logs");
mkdirSync(logDir, { recursive: true });
const logFile = resolve(logDir, "live-activity.log");

const ROUTER = process.env.ORDER_ROUTER_URL || "http://127.0.0.1:9098";
const SECRET = process.env.ORDER_ROUTER_RELAY_SECRET || "local-dev-secret";
const CAST = process.env.CAST_BIN || "C:\\Users\\Justin\\tools\\foundry\\cast.exe";
const PK =
  process.env.EVM_RELAY_EXECUTOR_PRIVATE_KEY ||
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const WALLET = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266";

function log(msg) {
  const line = `${new Date().toISOString()} ${msg}`;
  appendFileSync(logFile, line + "\n");
  console.log(line);
}

async function sleep(ms) {
  await new Promise((r) => setTimeout(r, ms));
}

async function call(path, { method = "GET", body, timeoutMs = 90_000 } = {}) {
  const headers = { "x-relay-secret": SECRET };
  if (body) headers["content-type"] = "application/json";
  const ac = new AbortController();
  const t = setTimeout(() => ac.abort(), timeoutMs);
  try {
    const res = await fetch(`${ROUTER}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
      signal: ac.signal,
    });
    const json = await res.json().catch(() => ({}));
    if (!res.ok || json.accepted === false) {
      throw new Error(`${method} ${path} ${res.status}: ${JSON.stringify(json)}`);
    }
    return json;
  } finally {
    clearTimeout(t);
  }
}

function envelope() {
  const env = randomBytes(128);
  return {
    envelope: "0x" + env.toString("hex"),
    commitment: "0x" + createHash("sha256").update(env).digest("hex"),
    nullifier: "0x" + randomBytes(32).toString("hex"),
  };
}

function signOrder(chainId, marketId, side, amountUsd, nonce) {
  const message = [
    "VEIL native order v1",
    `chain:${chainId}`,
    `market:${marketId}`,
    `side:${side}`,
    "outcome:yes",
    `amountUsd:${amountUsd.toFixed(8)}`,
    `wallet:${WALLET}`,
    `nonce:${nonce}`,
  ].join("\n");
  const signed = spawnSync(CAST, ["wallet", "sign", "--private-key", PK, message], {
    encoding: "utf8",
  });
  if (signed.status !== 0) {
    throw new Error(`cast sign: ${signed.stderr || signed.stdout}`);
  }
  return (signed.stdout || "").trim();
}

async function tickAmm(n) {
  const health = await fetch(`${ROUTER}/health`).then((r) => r.json()).catch(() => ({}))
  if (typeof health.veil === "number" && health.veil < 10_000) {
    log(`skip amm: actor VEIL ${health.veil} — faucet first`)
    return
  }
  // Spend VAI so ticks land without draining native float.
  const swap = await call("/intents/native/liquidity/execute", {
    method: "POST",
    body: {
      ...envelope(),
      operation: "swap_exact_in",
      assetIn: 1,
      assetOut: 0,
      amountIn: 4,
      minAmountOut: 1,
    },
  });
  log(`swap VAI->VEIL ${swap.veilTxHash}`);
  if (n > 12 && n % 11 === 0) {
    const add = await call("/intents/native/liquidity/execute", {
      method: "POST",
      body: {
        ...envelope(),
        operation: "add_liquidity",
        asset0: 0,
        asset1: 1,
        amount0: 2,
        amount1: 2,
        minLP: 1,
      },
    });
    log(`add_lp ${add.veilTxHash}`);
  }
}

async function tickMarket(n) {
  const health = await call("/health");
  if (!health.proverReady) {
    log("skip market: prover not ready");
    return;
  }
  const created = await call("/native/create-market", {
    method: "POST",
    body: {
      question: `live tape #${n} ${new Date().toISOString()}`,
      outcomes: 2,
      creatorBond: 1,
    },
  });
  const nonce = "0x" + randomBytes(32).toString("hex");
  const amountUsd = 8 + (n % 7);
  const side = n % 2 === 0 ? "buy" : "sell";
  const sig = signOrder(health.chainId, created.marketId, side, amountUsd, nonce);
  const order = await call("/orders", {
    method: "POST",
    body: {
      marketId: created.marketId,
      side,
      outcome: "yes",
      amountUsd,
      walletAddress: WALLET,
      walletNonce: nonce,
      walletSignature: sig,
      nativeNetwork: "veil",
      routingFeeBps: 0,
    },
  });
  log(`order ${order.veilTxHash} market=${created.marketId} window=${order.windowId}`);
  const settled = await call("/native/settle-batch", {
    method: "POST",
    timeoutMs: 120_000,
    body: {
      marketId: created.marketId,
      windowId: order.windowId,
      clearPrice: 1000 + (n % 40),
      totalVolume: 1,
    },
  });
  log(`clear ${settled.clearTxHash} proveMs=${settled.proveMs}`);
}

let n = 0;
log(`live-activity start pid=${process.pid}`);
while (true) {
  n += 1;
  try {
    await tickAmm(n);
    // create_market still posts a 1 VEIL creator bond; skip while native
    // balance is fee-dust. AMM VAI->VEIL is the live tape.
  } catch (err) {
    log(`tick ${n} FAIL ${err.message || err}`);
  }
  await sleep(12_000 + (n % 5) * 1000);
}
