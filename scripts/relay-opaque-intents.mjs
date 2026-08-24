#!/usr/bin/env node
/**
 * One-shot (default) or poll: companion gateway events -> order-router -> markIntentExecuted.
 * Envelope bytes come from VEIL_INTENT_MAILBOX_PATH JSON: { [commitment0x]: { envelope, ...hints } }
 */
import { createHash } from 'node:crypto';
import { readFileSync, existsSync } from 'node:fs';
import { spawnSync } from 'node:child_process';

const ROUTER = process.env.ORDER_ROUTER_URL || 'http://127.0.0.1:9098';
const SECRET = process.env.ORDER_ROUTER_RELAY_SECRET || 'local-dev-secret';
const EVM_RPC = process.env.EVM_RPC_URL || 'http://127.0.0.1:8545';
const ORDER_GW = process.env.ORDER_GATEWAY || '';
const LIQ_GW = process.env.LIQUIDITY_GATEWAY || '';
const MAILBOX = process.env.VEIL_INTENT_MAILBOX_PATH || new URL('./intent-mailbox.json', import.meta.url).pathname;
const PK = process.env.EVM_RELAY_EXECUTOR_PRIVATE_KEY || '';
const CAST = process.env.CAST_BIN || 'cast';
const fromBlock = Number(process.env.EVM_FROM_BLOCK || 0);

const ORDER_TOPIC = keccak(
  'IntentSubmitted(bytes32,bytes32,bytes32,bytes32,uint64)',
);
const LIQ_TOPIC = keccak(
  'LiquidityIntentSubmitted(bytes32,bytes32,bytes32,bytes32,uint64)',
);

function keccak(sig) {
  const r = spawnSync(CAST, ['sig-event', sig], { encoding: 'utf8' });
  if (r.status !== 0) {
    throw new Error(`cast sig-event failed: ${r.stderr || r.stdout}`);
  }
  return r.stdout.trim();
}

function loadMailbox() {
  const p = MAILBOX.replace(/^\/([A-Za-z]:)/, '$1');
  if (!existsSync(p)) return {};
  return JSON.parse(readFileSync(p, 'utf8'));
}

async function rpc(url, method, params) {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });
  const json = await res.json();
  if (json.error) throw new Error(`${method}: ${json.error.message}`);
  return json.result;
}

function padTopic(hex) {
  const h = hex.replace(/^0x/, '').toLowerCase();
  return '0x' + h.padStart(64, '0');
}

async function logs(address, topic) {
  if (!address) return [];
  return rpc(EVM_RPC, 'eth_getLogs', [
    {
      fromBlock: '0x' + fromBlock.toString(16),
      toBlock: 'latest',
      address,
      topics: [topic],
    },
  ]);
}

async function postRouter(path, body) {
  const res = await fetch(`${ROUTER}${path}`, {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      'x-relay-secret': SECRET,
    },
    body: JSON.stringify(body),
  });
  const json = await res.json();
  if (!res.ok || json.error) {
    throw new Error(`${path}: ${json.error || res.status}`);
  }
  return json;
}

function markExecuted(gateway, intentId, veilTxHash) {
  if (!PK) {
    console.log(`skip markIntentExecuted (no EVM_RELAY_EXECUTOR_PRIVATE_KEY) intent=${intentId}`);
    return;
  }
  const hash = veilTxHash.startsWith('0x') ? veilTxHash : `0x${veilTxHash}`;
  const r = spawnSync(
    CAST,
    [
      'send',
      gateway,
      'markIntentExecuted(bytes32,bytes32)',
      intentId,
      padTopic(hash),
      '--rpc-url',
      EVM_RPC,
      '--private-key',
      PK,
    ],
    { encoding: 'utf8' },
  );
  if (r.status !== 0) {
    throw new Error(`markIntentExecuted failed: ${r.stderr || r.stdout}`);
  }
  console.log(`marked executed ${intentId} tx=${(r.stdout || '').trim()}`);
}

function sha256Hex(buf) {
  return '0x' + createHash('sha256').update(buf).digest('hex');
}

function envelopeFromMailbox(mailbox, commitment) {
  const key = commitment.toLowerCase();
  const entry = mailbox[key] || mailbox[commitment] || mailbox[key.replace(/^0x/, '')];
  if (!entry) throw new Error(`mailbox miss for ${commitment}`);
  const envHex = typeof entry === 'string' ? entry : entry.envelope;
  const env = Buffer.from(envHex.replace(/^0x/, ''), 'hex');
  if (sha256Hex(env) !== key && sha256Hex(env) !== commitment.toLowerCase()) {
    throw new Error(`mailbox envelope hash mismatch for ${commitment}`);
  }
  return { envHex: envHex.startsWith('0x') ? envHex : `0x${envHex}`, hints: typeof entry === 'object' ? entry : {} };
}

async function relayOrder(mailbox, log) {
  const intentId = log.topics[1];
  const commitment = log.topics[2];
  const nullifier = log.topics[3];
  const { envHex, hints } = envelopeFromMailbox(mailbox, commitment);
  const body = {
    intentId,
    marketKey: hints.marketKey || hints.marketId,
    envelope: envHex,
    commitment,
    nullifier,
    marketType: hints.marketType || 'veil_native',
    routingFeeBps: hints.routingFeeBps || 0,
    windowId: hints.windowId || 1,
    sourceTxHash: log.transactionHash,
  };
  const out = await postRouter('/evm/intents/execute', body);
  console.log(`order relayed intent=${intentId} veil=${out.veilTxHash}`);
  markExecuted(ORDER_GW, intentId, out.veilTxHash);
  return out;
}

async function relayLiq(mailbox, log) {
  const intentId = log.topics[1];
  const commitment = log.topics[2];
  const nullifier = log.topics[3];
  const { envHex, hints } = envelopeFromMailbox(mailbox, commitment);
  const body = {
    intentId,
    envelope: envHex,
    commitment,
    nullifier,
    operation: hints.operation || 'add_liquidity',
    asset0: hints.asset0 ?? 0,
    asset1: hints.asset1 ?? 1,
    amount0: hints.amount0,
    amount1: hints.amount1,
    minLP: hints.minLP ?? 1,
    feeBips: hints.feeBips,
    sourceTxHash: log.transactionHash,
  };
  const out = await postRouter('/evm/liquidity/execute', body);
  console.log(`liq relayed intent=${intentId} veil=${out.veilTxHash}`);
  markExecuted(LIQ_GW, intentId, out.veilTxHash);
  return out;
}

async function once() {
  const mailbox = loadMailbox();
  let n = 0;
  for (const lg of await logs(ORDER_GW, ORDER_TOPIC)) {
    await relayOrder(mailbox, lg);
    n++;
  }
  for (const lg of await logs(LIQ_GW, LIQ_TOPIC)) {
    await relayLiq(mailbox, lg);
    n++;
  }
  console.log(`relayed ${n} intent(s)`);
  return n;
}

const watch = process.argv.includes('--watch');
if (watch) {
  const pollMs = Number(process.env.RELAY_POLL_MS || 2000);
  for (;;) {
    try {
      await once();
    } catch (e) {
      console.error(e.message || e);
    }
    await new Promise((r) => setTimeout(r, pollMs));
  }
} else {
  once().catch((e) => {
    console.error(e.message || e);
    process.exit(1);
  });
}
