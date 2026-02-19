#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const VEIL_VM_ID = 'u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK';
const scriptDir = dirname(fileURLToPath(import.meta.url));
const veilvmRoot = resolve(scriptDir, '..');
const hypersdkRoot = resolve(veilvmRoot, '..', '..');
const defaultPkPath = './zk-fixture-new/groth16_shielded_ledger_pk.bin';
const defaultDockerImage = 'veilvm-zkbench-evidence:local';
const defaultDockerfile = resolve(scriptDir, 'zkbench-runner.Dockerfile');
const defaultBenchPrivateKey =
  '637404e6722a0e55a27fd82dcd29f3f0faa6f13d930f32f759e3b8412c4956aeee9d3919f004304c2d44dbc9121f6559fefb9b9c25daec749b0f18f605614461';
const dockerWindowsFallback = 'C:\\Program Files\\Docker\\Docker\\resources\\bin\\docker.exe';
const cacheErrorPatterns = ['cannot allocate memory', '.partial', '/go/pkg/mod/cache/download'];
const dockerZkbenchBinaryName = 'veilvm-zkbench-linux-amd64';
const localZkbenchBinaryName = process.platform === 'win32' ? 'veilvm-zkbench.exe' : 'veilvm-zkbench';
const dockerProbeTimeoutMs = 30000;
const defaultPrefundAmount = 35_000_001;
const configuredVeilVmIds = String(process.env.VEIL_EVIDENCE_VM_IDS || VEIL_VM_ID)
  .split(',')
  .map((x) => x.trim())
  .filter(Boolean);

function printHelp() {
  console.log('VEIL launch-gate evidence bundle runner');
  console.log('');
  console.log('Usage:');
  console.log('  node run-launch-gate-evidence.mjs [options]');
  console.log('');
  console.log('Options:');
  console.log('  --node-url <URL>             AvalancheGo base URL (default: NODE_URL or http://127.0.0.1:9660)');
  console.log('  --chain-id <CHAIN_ID>        VEIL chain ID (auto-discovered if omitted)');
  console.log('  --pk-path <PATH>             Groth16 shielded proving key path');
  console.log('  --out-dir <PATH>             Evidence bundle output root');
  console.log('  --batch-size <N>             Shielded smoke batch size (default: 8)');
  console.log('  --windows-per-size <N>       Windows per size for smoke/negative (default: 1)');
  console.log('  --timeout-minutes <N>        zkbench timeout minutes (default: 20)');
  console.log('  --timeout-batches <LIST>     Timeout drill attempts, comma list (default: 8,32)');
  console.log('  --private-key <HEX>          Bench signer private key override');
  console.log('  --faucet-private-key <HEX>   Optional faucet key to top up run wallets before checks');
  console.log('  --backup-private-key <HEX>   Backup signer private key for takeover drill');
  console.log('  --proof-config-private-key <HEX>  Signer key for set_proof_config (defaults to genesis key)');
  console.log(
    `  --prefund-amount <N>         Prefund transfer amount before checks (default: ${defaultPrefundAmount})`,
  );
  console.log('  --skip-prefund-backup        Skip backup wallet prefund step');
  console.log('  --runner <docker|local>      Run zkbench in docker (default) or local go toolchain');
  console.log(`  --docker-image <TAG>         Docker image tag (default: ${defaultDockerImage})`);
  console.log(`  --dockerfile <PATH>          Dockerfile path (default: ${defaultDockerfile})`);
  console.log('  --skip-image-build           Do not build docker image when missing');
  console.log('  --skip-zkbench-prebuild      Skip binary prebuild and run via go run (slower)');
  console.log('  --preflight-only             Validate readiness and exit before running checks');
  console.log('  --skip-malformed             Skip malformed-proof drill');
  console.log('  --skip-backup-takeover       Skip backup takeover drill');
  console.log('  --skip-timeout               Skip timeout drill');
  console.log('  --skip-negative              Skip synthetic negative drill');
  console.log('  --help, -h                   Show this help');
  console.log('');
  console.log('Environment fallbacks:');
  console.log('  NODE_URL, CHAIN_ID, GROTH16_PK_PATH, PRIVATE_KEY, VEIL_EVIDENCE_OUT_DIR');
  console.log('  VEIL_EVIDENCE_BACKUP_PRIVATE_KEY, VEIL_EVIDENCE_FAUCET_PRIVATE_KEY');
  console.log('  VEIL_EVIDENCE_PROOF_CONFIG_PRIVATE_KEY');
  console.log('  VEIL_EVIDENCE_PREFUND_AMOUNT, VEIL_EVIDENCE_SKIP_PREFUND_BACKUP');
  console.log('  VEIL_EVIDENCE_RUNNER, VEIL_EVIDENCE_DOCKER_IMAGE, VEIL_EVIDENCE_DOCKERFILE');
  console.log('  VEIL_EVIDENCE_SKIP_ZKBENCH_PREBUILD');
  console.log('  VEIL_EVIDENCE_VM_IDS');
}

function parseIntStrict(value, name) {
  const n = Number.parseInt(String(value), 10);
  if (!Number.isFinite(n) || n <= 0) {
    throw new Error(`invalid ${name}: ${value}`);
  }
  return n;
}

function parseBatchList(raw) {
  const rows = String(raw)
    .split(',')
    .map((x) => Number.parseInt(x.trim(), 10))
    .filter((x) => Number.isFinite(x) && x > 0);
  if (!rows.length) {
    throw new Error(`invalid timeout batch list: ${raw}`);
  }
  return [...new Set(rows)];
}

function parseBool(raw, fallback = false) {
  if (raw == null || String(raw).trim() === '') {
    return fallback;
  }
  const value = String(raw).trim().toLowerCase();
  return value === '1' || value === 'true' || value === 'yes' || value === 'on';
}

function parseArgs(argv) {
  const envNodeUrl = String(process.env.NODE_URL || '').trim();
  const args = {
    nodeUrl: envNodeUrl || 'http://127.0.0.1:9660',
    nodeUrlExplicit: envNodeUrl.length > 0,
    chainId: process.env.CHAIN_ID || '',
    pkPath: process.env.GROTH16_PK_PATH || defaultPkPath,
    outDir: process.env.VEIL_EVIDENCE_OUT_DIR || resolve(veilvmRoot, 'evidence-bundles'),
    batchSize: parseIntStrict(process.env.VEIL_EVIDENCE_BATCH_SIZE || '8', 'VEIL_EVIDENCE_BATCH_SIZE'),
    windowsPerSize: parseIntStrict(
      process.env.VEIL_EVIDENCE_WINDOWS_PER_SIZE || '1',
      'VEIL_EVIDENCE_WINDOWS_PER_SIZE',
    ),
    timeoutMinutes: parseIntStrict(
      process.env.VEIL_EVIDENCE_TIMEOUT_MINUTES || '20',
      'VEIL_EVIDENCE_TIMEOUT_MINUTES',
    ),
    timeoutBatches: parseBatchList(process.env.VEIL_EVIDENCE_TIMEOUT_BATCHES || '8,32'),
    privateKey: process.env.PRIVATE_KEY || '',
    faucetPrivateKey: process.env.VEIL_EVIDENCE_FAUCET_PRIVATE_KEY || '',
    backupPrivateKey: process.env.VEIL_EVIDENCE_BACKUP_PRIVATE_KEY || '',
    proofConfigPrivateKey: process.env.VEIL_EVIDENCE_PROOF_CONFIG_PRIVATE_KEY || '',
    prefundAmount: parseIntStrict(
      process.env.VEIL_EVIDENCE_PREFUND_AMOUNT || String(defaultPrefundAmount),
      'VEIL_EVIDENCE_PREFUND_AMOUNT',
    ),
    skipPrefundBackup: parseBool(process.env.VEIL_EVIDENCE_SKIP_PREFUND_BACKUP, false),
    runner: String(process.env.VEIL_EVIDENCE_RUNNER || 'docker').toLowerCase(),
    dockerImage: process.env.VEIL_EVIDENCE_DOCKER_IMAGE || defaultDockerImage,
    dockerfile: process.env.VEIL_EVIDENCE_DOCKERFILE || defaultDockerfile,
    skipImageBuild: false,
    skipZkbenchPrebuild: parseBool(process.env.VEIL_EVIDENCE_SKIP_ZKBENCH_PREBUILD, false),
    preflightOnly: false,
    skipMalformed: false,
    skipBackupTakeover: false,
    skipTimeout: false,
    skipNegative: false,
  };

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--node-url' && argv[i + 1]) {
      args.nodeUrl = argv[++i];
      args.nodeUrlExplicit = true;
      continue;
    }
    if (a === '--chain-id' && argv[i + 1]) {
      args.chainId = argv[++i];
      continue;
    }
    if (a === '--pk-path' && argv[i + 1]) {
      args.pkPath = argv[++i];
      continue;
    }
    if (a === '--out-dir' && argv[i + 1]) {
      args.outDir = argv[++i];
      continue;
    }
    if (a === '--batch-size' && argv[i + 1]) {
      args.batchSize = parseIntStrict(argv[++i], '--batch-size');
      continue;
    }
    if (a === '--windows-per-size' && argv[i + 1]) {
      args.windowsPerSize = parseIntStrict(argv[++i], '--windows-per-size');
      continue;
    }
    if (a === '--timeout-minutes' && argv[i + 1]) {
      args.timeoutMinutes = parseIntStrict(argv[++i], '--timeout-minutes');
      continue;
    }
    if (a === '--timeout-batches' && argv[i + 1]) {
      args.timeoutBatches = parseBatchList(argv[++i]);
      continue;
    }
    if (a === '--private-key' && argv[i + 1]) {
      args.privateKey = argv[++i];
      continue;
    }
    if (a === '--faucet-private-key' && argv[i + 1]) {
      args.faucetPrivateKey = argv[++i];
      continue;
    }
    if (a === '--backup-private-key' && argv[i + 1]) {
      args.backupPrivateKey = argv[++i];
      continue;
    }
    if (a === '--proof-config-private-key' && argv[i + 1]) {
      args.proofConfigPrivateKey = argv[++i];
      continue;
    }
    if (a === '--prefund-amount' && argv[i + 1]) {
      args.prefundAmount = parseIntStrict(argv[++i], '--prefund-amount');
      continue;
    }
    if (a === '--skip-prefund-backup') {
      args.skipPrefundBackup = true;
      continue;
    }
    if (a === '--runner' && argv[i + 1]) {
      args.runner = String(argv[++i]).toLowerCase();
      continue;
    }
    if (a === '--docker-image' && argv[i + 1]) {
      args.dockerImage = argv[++i];
      continue;
    }
    if (a === '--dockerfile' && argv[i + 1]) {
      args.dockerfile = argv[++i];
      continue;
    }
    if (a === '--skip-image-build') {
      args.skipImageBuild = true;
      continue;
    }
    if (a === '--skip-zkbench-prebuild') {
      args.skipZkbenchPrebuild = true;
      continue;
    }
    if (a === '--preflight-only') {
      args.preflightOnly = true;
      continue;
    }
    if (a === '--skip-malformed') {
      args.skipMalformed = true;
      continue;
    }
    if (a === '--skip-backup-takeover') {
      args.skipBackupTakeover = true;
      continue;
    }
    if (a === '--skip-timeout') {
      args.skipTimeout = true;
      continue;
    }
    if (a === '--skip-negative') {
      args.skipNegative = true;
      continue;
    }
    if (a === '--help' || a === '-h') {
      printHelp();
      process.exit(0);
    }
    throw new Error(`unknown arg: ${a}`);
  }

  if (args.runner !== 'docker' && args.runner !== 'local') {
    throw new Error(`invalid --runner value: ${args.runner} (expected docker|local)`);
  }

  return args;
}

function isPrivateKeyHex(value) {
  return /^[0-9a-fA-F]{128}$/.test(String(value || '').trim());
}

function requirePrivateKeyHex(name, value) {
  if (!isPrivateKeyHex(value)) {
    throw new Error(`${name} must be 128 hex chars`);
  }
  return String(value).trim();
}

function extractPrivateKeyFromKeygenOutput(text) {
  const match = String(text || '').match(/Private Key \(hex\):\s*([0-9a-fA-F]{128})/);
  return match?.[1] ? match[1].toLowerCase() : '';
}

function generateBackupPrivateKey({ runner, dockerImage, goModCacheDir, goBuildCacheDir }) {
  if (runner === 'docker') {
    const child = runDocker(
      [
        'run',
        '--rm',
        '-v',
        `${hypersdkRoot}:/workspace`,
        '-v',
        `${goModCacheDir}:/go/pkg/mod`,
        '-v',
        `${goBuildCacheDir}:/root/.cache/go-build`,
        '-w',
        '/workspace/examples/veilvm',
        '-e',
        'CGO_ENABLED=1',
        '-e',
        'GOMODCACHE=/go/pkg/mod',
        '-e',
        'GOCACHE=/root/.cache/go-build',
        dockerImage,
        'bash',
        '-lc',
        'export PATH=/usr/local/go/bin:$PATH && go run ./cmd/veilvm-keygen 2>&1',
      ],
      { cwd: veilvmRoot, env: process.env, timeout: 1000 * 60 * 5 },
    );
    const output = commandOutput(child);
    if (commandFailed(child)) {
      throw new Error(`failed to generate backup key in docker\n${commandFailureDetails('keygen', child)}`);
    }
    const key = extractPrivateKeyFromKeygenOutput(output);
    if (!key) {
      throw new Error('failed to parse backup key from keygen output (docker)');
    }
    return key;
  }

  const child = runCommand('go', ['run', './cmd/veilvm-keygen'], {
    cwd: veilvmRoot,
    env: process.env,
    timeout: 1000 * 60 * 5,
  });
  const output = commandOutput(child);
  if (commandFailed(child)) {
    throw new Error(`failed to generate backup key\n${commandFailureDetails('go run ./cmd/veilvm-keygen', child)}`);
  }
  const key = extractPrivateKeyFromKeygenOutput(output);
  if (!key) {
    throw new Error('failed to parse backup key from keygen output');
  }
  return key;
}

function isoStampCompact(date = new Date()) {
  const pad = (n) => String(n).padStart(2, '0');
  return (
    `${date.getUTCFullYear()}${pad(date.getUTCMonth() + 1)}${pad(date.getUTCDate())}-` +
    `${pad(date.getUTCHours())}${pad(date.getUTCMinutes())}${pad(date.getUTCSeconds())}`
  );
}

function sleep(ms) {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, ms));
}

async function fetchWithTimeout(url, options = {}, timeoutMs = 10000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } finally {
    clearTimeout(timer);
  }
}

async function jsonRpc(url, method, params = {}) {
  const response = await fetchWithTimeout(
    url,
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
    },
    10000,
  );
  if (!response.ok) {
    throw new Error(`${method}: HTTP ${response.status}`);
  }
  const payload = await response.json();
  if (payload?.error) {
    throw new Error(`${method}: ${payload.error.message}`);
  }
  return payload.result;
}

function runCommand(command, args, options = {}) {
  const child = spawnSync(command, args, {
    ...options,
    encoding: options.encoding || 'utf8',
    maxBuffer: options.maxBuffer || 1024 * 1024 * 256,
  });
  return child;
}

function runDocker(args, options = {}) {
  let child = runCommand('docker', args, options);
  if (child.error && child.error.code === 'ENOENT' && process.platform === 'win32') {
    child = runCommand(dockerWindowsFallback, args, options);
  }
  return child;
}

function commandFailed(child) {
  return Boolean(child.error) || Number(child.status) !== 0;
}

function commandOutput(child) {
  return `${child.stdout || ''}\n${child.stderr || ''}`.trim();
}

function commandFailureDetails(command, child) {
  if (child.error) {
    return `${command}: ${child.error.message || child.error}`;
  }
  return `${command}: exit ${child.status}\n${commandOutput(child)}`;
}

function toDockerReachableNodeUrl(originUrl) {
  let parsed;
  try {
    parsed = new URL(originUrl);
  } catch {
    return originUrl;
  }
  const host = String(parsed.hostname || '').toLowerCase();
  if (host === '127.0.0.1' || host === 'localhost' || host === '::1') {
    parsed.hostname = 'host.docker.internal';
  }
  return parsed.origin;
}

function toContainerPath(hostPath) {
  const rel = relative(hypersdkRoot, hostPath);
  if (rel.startsWith('..')) {
    throw new Error(`path is outside hypersdk root and cannot be mounted: ${hostPath}`);
  }
  return `/workspace/${rel.replaceAll('\\', '/')}`;
}

function ensureDockerImage(args) {
  const probe = runDocker(['version', '--format', '{{.Server.Version}}'], {
    cwd: veilvmRoot,
    env: process.env,
    timeout: dockerProbeTimeoutMs,
  });
  if (commandFailed(probe)) {
    throw new Error(
      `docker daemon not reachable; start Docker Desktop and retry\n${commandFailureDetails(
        'docker version',
        probe,
      )}`,
    );
  }
  const dockerServerVersion = String(probe.stdout || '').trim();

  const inspect = runDocker(['image', 'inspect', args.dockerImage], {
    cwd: veilvmRoot,
    env: process.env,
    timeout: dockerProbeTimeoutMs,
  });
  if (inspect.status === 0) {
    return { built: false, stdout: inspect.stdout || '', dockerServerVersion };
  }
  if (args.skipImageBuild) {
    throw new Error(
      `docker image not found and --skip-image-build was set: ${args.dockerImage}`,
    );
  }
  if (!existsSync(args.dockerfile)) {
    throw new Error(`dockerfile not found: ${args.dockerfile}`);
  }
  const build = runDocker(['build', '-f', args.dockerfile, '-t', args.dockerImage, scriptDir], {
    cwd: veilvmRoot,
    env: process.env,
    timeout: 1000 * 60 * 30,
  });
  if (build.status !== 0) {
    const out = `${build.stdout || ''}\n${build.stderr || ''}`;
    throw new Error(`docker build failed for ${args.dockerImage}\n${out}`);
  }
  return { built: true, stdout: build.stdout || '', dockerServerVersion };
}

function runDockerBuildCommand({
  dockerImage,
  goModCacheDir,
  goBuildCacheDir,
  buildCommand,
}) {
  return runDocker(
    [
      'run',
      '--rm',
      '-v',
      `${hypersdkRoot}:/workspace`,
      '-v',
      `${goModCacheDir}:/go/pkg/mod`,
      '-v',
      `${goBuildCacheDir}:/root/.cache/go-build`,
      '-w',
      '/workspace/examples/veilvm',
      '-e',
      'CGO_ENABLED=1',
      '-e',
      'GOMODCACHE=/go/pkg/mod',
      '-e',
      'GOCACHE=/root/.cache/go-build',
      dockerImage,
      'bash',
      '-lc',
      buildCommand,
    ],
    {
      cwd: veilvmRoot,
      env: process.env,
      timeout: 1000 * 60 * 20,
    },
  );
}

function prebuildZkbench({
  runner,
  dockerImage,
  goModCacheDir,
  goBuildCacheDir,
}) {
  const binDir = resolve(veilvmRoot, '.cache', 'evidence-zkbench', 'bin');
  mkdirSync(binDir, { recursive: true });

  if (runner === 'docker') {
    const hostBinPath = resolve(binDir, dockerZkbenchBinaryName);
    const containerBinPath = toContainerPath(hostBinPath);
    const buildCommand = [
      'export PATH=/usr/local/go/bin:$PATH',
      'mkdir -p /workspace/examples/veilvm/.cache/evidence-zkbench/bin',
      `go build -o ${containerBinPath} ./cmd/veilvm-zkbench`,
    ].join(' && ');
    const runOnce = () =>
      runDockerBuildCommand({
        dockerImage,
        goModCacheDir,
        goBuildCacheDir,
        buildCommand,
      });

    let child = runOnce();
    let recoveredAfterCacheReset = false;
    if (
      child.status !== 0 &&
      containsAny(`${child.stdout || ''}\n${child.stderr || ''}`.toLowerCase(), cacheErrorPatterns)
    ) {
      rmSync(goModCacheDir, { recursive: true, force: true });
      mkdirSync(goModCacheDir, { recursive: true });
      child = runOnce();
      recoveredAfterCacheReset = child.status === 0;
    }
    if (child.status !== 0) {
      const out = `${child.stdout || ''}\n${child.stderr || ''}`;
      throw new Error(`failed to prebuild zkbench binary in docker\n${out}`);
    }
    return {
      mode: 'binary',
      binaryPath: containerBinPath,
      recoveredAfterCacheReset,
    };
  }

  const localBinPath = resolve(binDir, localZkbenchBinaryName);
  const child = runCommand('go', ['build', '-o', localBinPath, './cmd/veilvm-zkbench'], {
    cwd: veilvmRoot,
    env: process.env,
  });
  if (child.status !== 0) {
    const out = `${child.stdout || ''}\n${child.stderr || ''}`;
    throw new Error(`failed to prebuild zkbench binary locally\n${out}`);
  }
  return {
    mode: 'binary',
    binaryPath: localBinPath,
    recoveredAfterCacheReset: false,
  };
}

async function waitForHealthy(nodeUrl, maxSeconds = 45) {
  const deadlineMs = Date.now() + maxSeconds * 1000;
  while (Date.now() < deadlineMs) {
    const remainingMs = deadlineMs - Date.now();
    const reqTimeoutMs = Math.max(1000, Math.min(4000, remainingMs));
    try {
      const readinessRes = await fetchWithTimeout(`${nodeUrl}/ext/health/readiness`, {}, reqTimeoutMs);
      if (readinessRes.ok) {
        const readinessPayload = await readinessRes.json();
        if (readinessPayload?.healthy) {
          return { ...readinessPayload, source: 'readiness' };
        }
      }
    } catch {}
    try {
      const healthRes = await fetchWithTimeout(`${nodeUrl}/ext/health`, {}, reqTimeoutMs);
      if (healthRes.ok) {
        const healthPayload = await healthRes.json();
        if (healthPayload?.healthy) {
          return { ...healthPayload, source: 'health' };
        }
      }
    } catch {}
    const sleepMs = Math.max(250, Math.min(1000, deadlineMs - Date.now()));
    await sleep(sleepMs);
  }
  throw new Error(`node did not become healthy within ${maxSeconds}s: ${nodeUrl}`);
}

async function resolveHealthyNodeUrl(args) {
  const candidates = [args.nodeUrl];
  if (!args.nodeUrlExplicit && String(args.nodeUrl) === 'http://127.0.0.1:9660') {
    candidates.push('http://127.0.0.1:9650');
  }

  let lastErr = null;
  for (const candidate of candidates) {
    try {
      await waitForHealthy(candidate);
      return candidate;
    } catch (err) {
      lastErr = err;
    }
  }
  const lastMessage = lastErr?.message || 'unknown error';
  const troubleshooting = [
    `node did not become healthy after trying: ${candidates.join(', ')}`,
    `last error: ${lastMessage}`,
    'troubleshooting:',
    '- start/restart VeilVM node: docker compose -f docker-compose.local.yml up -d --build node',
    '- ensure shielded verifier gate is active: VEIL_ZK_REQUIRED_CIRCUIT_ID=shielded-ledger-v1',
    '- verify readiness endpoint: http://127.0.0.1:9660/ext/health/readiness',
    '- if docker commands hang, restart Docker Desktop and retry',
  ].join('\n');
  throw new Error(troubleshooting);
}

async function discoverVeilChain(nodeUrl) {
  const result = await jsonRpc(`${nodeUrl}/ext/bc/P`, 'platform.getBlockchains', {});
  const rows = Array.isArray(result?.blockchains) ? result.blockchains : [];
  const vmMatchedRows = rows.filter((row) => configuredVeilVmIds.includes(String(row?.vmID || '')));
  const veilRows = vmMatchedRows.length
    ? vmMatchedRows
    : rows.filter((row) => String(row?.name || '').toUpperCase().startsWith('VEIL'));
  if (!veilRows.length) {
    throw new Error(`no VEIL VM chains found on ${nodeUrl}`);
  }
  const preferred =
    veilRows.find((row) => String(row?.name || '').toUpperCase() === 'VEIL') ||
    veilRows.find((row) => String(row?.name || '').toUpperCase() === 'VEIL2') ||
    veilRows[0];
  return {
    chainId: String(preferred?.id || ''),
    strategy: vmMatchedRows.length ? 'vm-id' : 'name-fallback',
    discovered: veilRows.map((row) => ({
      id: String(row?.id || ''),
      name: String(row?.name || ''),
      subnetID: String(row?.subnetID || ''),
      vmID: String(row?.vmID || ''),
    })),
  };
}

function short(value) {
  const s = String(value || '');
  if (s.length <= 20) return s;
  return `${s.slice(0, 10)}...${s.slice(-8)}`;
}

function primarySummary(run) {
  return run?.summary?.results?.[0]?.summary || null;
}

function errorText(run) {
  return String(run?.errorSnippet || '').toLowerCase();
}

function containsAny(text, patterns) {
  return patterns.some((pattern) => text.includes(String(pattern).toLowerCase()));
}

function evaluateShieldedSmoke(run) {
  if (run.exitCode !== 0) {
    return { pass: false, reason: `process exit code ${run.exitCode}` };
  }
  const s = primarySummary(run);
  if (!s) return { pass: false, reason: 'missing summary payload' };
  const accepted = Number(s.total_accepted_batches || 0);
  const rejected = Number(s.total_rejected_batches || 0);
  const missed = Number(s.total_missed_proof_deadlines || 0);
  if (accepted < 1) return { pass: false, reason: `accepted=${accepted} (expected >=1)` };
  if (rejected !== 0) return { pass: false, reason: `rejected=${rejected} (expected 0)` };
  if (missed !== 0) return { pass: false, reason: `missed_deadlines=${missed} (expected 0)` };
  return { pass: true, reason: `accepted=${accepted}, rejected=${rejected}, missed=${missed}` };
}

function evaluateSyntheticNegative(run) {
  if (run.exitCode !== 0) {
    const text = errorText(run);
    if (
      containsAny(text, [
        'proof verification failed',
        'proof circuit mismatch',
        'submit_batch_proof execution failed',
      ])
    ) {
      return { pass: true, reason: 'expected fail-close proof rejection observed (non-zero exit)' };
    }
    return { pass: false, reason: `unexpected process exit code ${run.exitCode}` };
  }
  const s = primarySummary(run);
  if (!s) return { pass: false, reason: 'missing summary payload' };
  const accepted = Number(s.total_accepted_batches || 0);
  const rejected = Number(s.total_rejected_batches || 0);
  if (accepted !== 0) return { pass: false, reason: `accepted=${accepted} (expected 0)` };
  if (rejected < 1) return { pass: false, reason: `rejected=${rejected} (expected >=1)` };
  return { pass: true, reason: `accepted=${accepted}, rejected=${rejected}` };
}

function evaluateMalformedProof(run) {
  if (run.exitCode !== 0) {
    const text = errorText(run);
    if (
      containsAny(text, [
        'invalid proof envelope',
        'proof verification failed',
        'failed to parse proof',
        'deserialize',
        'submit_batch_proof execution failed',
      ])
    ) {
      return { pass: true, reason: 'expected malformed-proof rejection observed (non-zero exit)' };
    }
    return { pass: false, reason: `unexpected process exit code ${run.exitCode}` };
  }
  const s = primarySummary(run);
  if (!s) return { pass: false, reason: 'missing summary payload' };
  const accepted = Number(s.total_accepted_batches || 0);
  const rejected = Number(s.total_rejected_batches || 0);
  if (accepted !== 0) return { pass: false, reason: `accepted=${accepted} (expected 0)` };
  if (rejected < 1) return { pass: false, reason: `rejected=${rejected} (expected >=1)` };
  return { pass: true, reason: `accepted=${accepted}, rejected=${rejected}` };
}

function evaluateBackupPrimaryFailure(run) {
  if (run.exitCode !== 0) {
    const text = errorText(run);
    if (
      containsAny(text, [
        'unauthorized',
        'prover authority',
        'submit_batch_proof execution failed',
      ])
    ) {
      return { pass: true, reason: 'primary prover rejected under backup authority gate' };
    }
    return { pass: false, reason: `unexpected process exit code ${run.exitCode}` };
  }
  return { pass: false, reason: 'primary prover unexpectedly succeeded' };
}

function evaluateTimeoutDrill(run) {
  if (run.exitCode !== 0) {
    const text = errorText(run);
    if (
      containsAny(text, [
        'missed proof deadline',
        'proof deadline',
        'window close',
        'submit_batch_proof execution failed',
      ])
    ) {
      return { pass: true, reason: 'expected timeout/deadline failure observed (non-zero exit)' };
    }
    return { pass: false, reason: `unexpected process exit code ${run.exitCode}` };
  }
  const s = primarySummary(run);
  if (!s) return { pass: false, reason: 'missing summary payload' };
  const rejected = Number(s.total_rejected_batches || 0);
  const missed = Number(s.total_missed_proof_deadlines || 0);
  if (missed > 0 || rejected > 0) {
    return { pass: true, reason: `rejected=${rejected}, missed_deadlines=${missed}` };
  }
  return { pass: false, reason: 'no missed deadline or rejection observed' };
}

function pushDockerEnvVar(dockerArgs, env, key) {
  if (!(key in env)) return;
  const value = env[key];
  if (value == null || String(value) === '') return;
  dockerArgs.push('-e', `${key}=${value}`);
}

function runZkbenchCase({
  caseName,
  bundleDir,
  bundleStamp,
  baseEnv,
  envOverrides,
  runner,
  dockerImage,
  goModCacheDir,
  goBuildCacheDir,
  pkHostPath,
  zkbenchExec,
}) {
  const outputDir = `./zkbench-out-evidence-${bundleStamp}-${caseName}`;
  const runEnv = {
    ...baseEnv,
    ...envOverrides,
    OUTPUT_DIR: outputDir,
  };
  const logsDir = resolve(bundleDir, 'logs');
  mkdirSync(logsDir, { recursive: true });
  const stdoutLog = resolve(logsDir, `${caseName}.stdout.log`);
  const stderrLog = resolve(logsDir, `${caseName}.stderr.log`);

  function executeOnce() {
    if (runner === 'docker') {
      const dockerEnv = {
        ...runEnv,
        NODE_URL: toDockerReachableNodeUrl(runEnv.NODE_URL),
        GROTH16_PK_PATH: runEnv.GROTH16_PK_PATH ? toContainerPath(pkHostPath) : '',
      };
      const dockerArgs = [
        'run',
        '--rm',
        '-v',
        `${hypersdkRoot}:/workspace`,
        '-v',
        `${goModCacheDir}:/go/pkg/mod`,
        '-v',
        `${goBuildCacheDir}:/root/.cache/go-build`,
        '-w',
        '/workspace/examples/veilvm',
        '-e',
        `NODE_URL=${dockerEnv.NODE_URL}`,
        '-e',
        `CHAIN_ID=${dockerEnv.CHAIN_ID}`,
        '-e',
        `PROOF_MODE=${dockerEnv.PROOF_MODE}`,
        '-e',
        `PROOF_CIRCUIT_ID=${dockerEnv.PROOF_CIRCUIT_ID}`,
        '-e',
        `BATCH_SIZES=${dockerEnv.BATCH_SIZES}`,
        '-e',
        `WINDOWS_PER_SIZE=${dockerEnv.WINDOWS_PER_SIZE}`,
        '-e',
        `BATCH_WINDOW_MS=${dockerEnv.BATCH_WINDOW_MS}`,
        '-e',
        `PROOF_DEADLINE_MS=${dockerEnv.PROOF_DEADLINE_MS}`,
        '-e',
        `TIMEOUT_MINUTES=${dockerEnv.TIMEOUT_MINUTES}`,
        '-e',
        `OUTPUT_DIR=${dockerEnv.OUTPUT_DIR}`,
        '-e',
        'CGO_ENABLED=1',
        '-e',
        'GOMODCACHE=/go/pkg/mod',
        '-e',
        'GOCACHE=/root/.cache/go-build',
      ];
      pushDockerEnvVar(dockerArgs, dockerEnv, 'GROTH16_PK_PATH');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'PRIVATE_KEY');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'REFUEL_PRIVATE_KEY');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'PROVER_AUTHORITY_PRIVATE_KEY');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'PROOF_CONFIG_PRIVATE_KEY');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'PROOF_TAMPER_MODE');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'GROTH16_CCS_CACHE_PATH');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'STRICT_FEE_PREFLIGHT');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'GAS_SAFETY_BPS');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'GAS_RESERVE');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'REFUEL_AMOUNT');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'PROOF_SUBMIT_DELAY_MS');
      pushDockerEnvVar(dockerArgs, dockerEnv, 'PREFUND_ONLY');
      if (zkbenchExec?.mode === 'binary' && zkbenchExec.binaryPath) {
        dockerArgs.push(dockerImage, zkbenchExec.binaryPath);
      } else {
        dockerArgs.push(
          dockerImage,
          'bash',
          '-lc',
          'export PATH=/usr/local/go/bin:$PATH && go run ./cmd/veilvm-zkbench',
        );
      }
      return runDocker(dockerArgs, {
        cwd: veilvmRoot,
        env: process.env,
      });
    }
    if (zkbenchExec?.mode === 'binary' && zkbenchExec.binaryPath) {
      return runCommand(zkbenchExec.binaryPath, [], {
        cwd: veilvmRoot,
        env: runEnv,
      });
    }
    return runCommand('go', ['run', './cmd/veilvm-zkbench'], {
      cwd: veilvmRoot,
      env: runEnv,
    });
  }

  function resultFromChild(child, startedMs, startedAt, retries = 0, recovered = false) {
    const endedAt = new Date().toISOString();
    const durationMs = Date.now() - startedMs;
    const stdout = child.stdout || '';
    const stderr = child.stderr || '';
    writeFileSync(stdoutLog, stdout, 'utf8');
    writeFileSync(stderrLog, stderr, 'utf8');

    const summaryPath = resolve(veilvmRoot, outputDir, 'summary.json');
    let summary = null;
    if (existsSync(summaryPath)) {
      try {
        summary = JSON.parse(readFileSync(summaryPath, 'utf8'));
      } catch {}
    }

    const result = {
      name: caseName,
      startedAt,
      endedAt,
      durationMs,
      exitCode: Number.isFinite(child.status) ? child.status : -1,
      signal: child.signal || '',
      outputDir,
      summaryPath: existsSync(summaryPath) ? summaryPath : '',
      summary,
      retries,
      recoveredAfterCacheReset: recovered,
      logs: {
        stdout: stdoutLog,
        stderr: stderrLog,
      },
    };
    if (result.exitCode !== 0) {
      result.errorSnippet = [stdout, stderr]
        .join('\n')
        .split(/\r?\n/)
        .slice(-80)
        .join('\n');
    }
    return result;
  }

  const startedAt = new Date().toISOString();
  const startedMs = Date.now();
  const first = executeOnce();
  let result = resultFromChild(first, startedMs, startedAt);

  if (
    runner === 'docker' &&
    result.exitCode !== 0 &&
    containsAny(errorText(result), cacheErrorPatterns)
  ) {
    rmSync(goModCacheDir, { recursive: true, force: true });
    mkdirSync(goModCacheDir, { recursive: true });
    const second = executeOnce();
    result = resultFromChild(second, startedMs, startedAt, 1, second.status === 0);
  }

  return result;
}

function toRel(pathValue) {
  if (!pathValue) return '';
  return relative(veilvmRoot, pathValue) || '.';
}

function formatCaseMarkdown(run, evaluation) {
  const s = primarySummary(run);
  const accepted = Number(s?.total_accepted_batches || 0);
  const rejected = Number(s?.total_rejected_batches || 0);
  const missed = Number(s?.total_missed_proof_deadlines || 0);
  return [
    `| ${run.name} | ${evaluation.pass ? 'PASS' : 'FAIL'} | ${(run.durationMs / 1000).toFixed(1)} | ${accepted} | ${rejected} | ${missed} | \`${run.outputDir}\` | ${evaluation.reason} |`,
  ].join('');
}

function writeBundleMarkdown(bundlePath, payload) {
  const lines = [];
  lines.push('# VEIL Launch-Gate Evidence Bundle');
  lines.push('');
  lines.push(`- Generated: ${payload.generatedAt}`);
  lines.push(`- Node URL: \`${payload.nodeUrl}\``);
  lines.push(`- Chain ID: \`${payload.chainId}\``);
  lines.push(`- Verdict: **${payload.overallPass ? 'PASS' : 'FAIL'}**`);
  lines.push('');
  lines.push('## Checks');
  lines.push('');
  lines.push('| Check | Status | Duration (s) | Accepted | Rejected | Missed | Output Dir | Notes |');
  lines.push('|---|---|---:|---:|---:|---:|---|---|');
  for (const check of payload.checks) {
    for (const attempt of check.attempts) {
      lines.push(formatCaseMarkdown(attempt.run, attempt.evaluation));
    }
  }
  lines.push('');
  lines.push('## Artifacts');
  lines.push('');
  for (const check of payload.checks) {
    for (const attempt of check.attempts) {
      lines.push(`- ${attempt.run.name}`);
      lines.push(`  - summary: \`${toRel(attempt.run.summaryPath)}\``);
      lines.push(`  - stdout: \`${toRel(attempt.run.logs.stdout)}\``);
      lines.push(`  - stderr: \`${toRel(attempt.run.logs.stderr)}\``);
    }
  }
  lines.push('');
  writeFileSync(bundlePath, `${lines.join('\n')}\n`, 'utf8');
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const requestedNodeUrl = args.nodeUrl;
  args.nodeUrl = await resolveHealthyNodeUrl(args);
  if (args.nodeUrl !== requestedNodeUrl) {
    console.log(`Node URL fallback: ${requestedNodeUrl} -> ${args.nodeUrl}`);
  }

  let chainId = args.chainId;
  let discoveredChains = [];
  let chainDiscoveryStrategy = args.chainId ? 'explicit' : '';
  if (!chainId) {
    const discovered = await discoverVeilChain(args.nodeUrl);
    chainId = discovered.chainId;
    discoveredChains = discovered.discovered;
    chainDiscoveryStrategy = discovered.strategy;
  }
  if (!chainId) {
    throw new Error('failed to resolve chain ID');
  }

  const pkHostPath = resolve(veilvmRoot, args.pkPath);
  if (!existsSync(pkHostPath)) {
    throw new Error(`shielded proving key not found: ${pkHostPath}`);
  }

  const bundleStamp = isoStampCompact();
  const bundleDir = resolve(args.outDir, `${bundleStamp}-launch-gate-evidence`);
  mkdirSync(bundleDir, { recursive: true });
  mkdirSync(resolve(bundleDir, 'logs'), { recursive: true });
  const goModCacheDir = resolve(veilvmRoot, '.cache', 'evidence-zkbench', 'go-mod');
  const goBuildCacheDir = resolve(veilvmRoot, '.cache', 'evidence-zkbench', 'go-build');
  mkdirSync(goModCacheDir, { recursive: true });
  mkdirSync(goBuildCacheDir, { recursive: true });

  let dockerBuilt = false;
  let dockerServerVersion = '';
  if (args.runner === 'docker') {
    const dockerStatus = ensureDockerImage(args);
    dockerBuilt = Boolean(dockerStatus.built);
    dockerServerVersion = String(dockerStatus.dockerServerVersion || '');
  }
  let zkbenchExec = { mode: 'go-run', binaryPath: '', recoveredAfterCacheReset: false };
  if (!args.skipZkbenchPrebuild) {
    zkbenchExec = prebuildZkbench({
      runner: args.runner,
      dockerImage: args.dockerImage,
      goModCacheDir,
      goBuildCacheDir,
    });
  }

  console.log(`Node URL: ${args.nodeUrl}`);
  console.log(`Chain ID: ${chainId} (${short(chainId)})`);
  if (chainDiscoveryStrategy) {
    console.log(`Chain discovery: ${chainDiscoveryStrategy}`);
  }
  console.log(`Runner: ${args.runner}`);
  if (args.runner === 'docker') {
    console.log(`Docker image: ${args.dockerImage}${dockerBuilt ? ' (built)' : ' (reused)'}`);
    console.log(`Docker server: ${dockerServerVersion || 'unknown'}`);
  }
  console.log(
    `zkbench exec: ${
      zkbenchExec.mode === 'binary' ? `prebuilt binary (${zkbenchExec.binaryPath})` : 'go run'
    }`,
  );
  console.log(`Bundle dir: ${bundleDir}`);

  if (args.preflightOnly) {
    console.log('Preflight: PASS (node, chain, proving key, docker/image, zkbench prebuild)');
    return;
  }

  const primaryPrivateKey = requirePrivateKeyHex(
    'primary private key',
    args.privateKey || defaultBenchPrivateKey,
  );
  const proofConfigPrivateKey = requirePrivateKeyHex(
    'proof-config private key',
    args.proofConfigPrivateKey || defaultBenchPrivateKey,
  );
  const faucetPrivateKey = args.faucetPrivateKey
    ? requirePrivateKeyHex('faucet private key', args.faucetPrivateKey)
    : '';
  const baseEnv = {
    ...process.env,
    NODE_URL: args.nodeUrl,
    CHAIN_ID: chainId,
    TIMEOUT_MINUTES: String(args.timeoutMinutes),
    PRIVATE_KEY: primaryPrivateKey,
    PROOF_CONFIG_PRIVATE_KEY: proofConfigPrivateKey,
  };
  let backupPrivateKey = '';
  if (!args.skipBackupTakeover) {
    if (args.backupPrivateKey) {
      backupPrivateKey = requirePrivateKeyHex(
        'backup private key',
        args.backupPrivateKey,
      );
    } else {
      console.log('Generating backup private key for takeover drill...');
      backupPrivateKey = generateBackupPrivateKey({
        runner: args.runner,
        dockerImage: args.dockerImage,
        goModCacheDir,
        goBuildCacheDir,
      });
      console.log(`Backup key generated: ${short(backupPrivateKey)}`);
    }
    if (backupPrivateKey === primaryPrivateKey) {
      throw new Error('backup private key must differ from primary private key');
    }
  }

  const ensureCaseSuccess = (label, run) => {
    if (run.exitCode === 0) return;
    throw new Error(
      `${label} failed (exit=${run.exitCode}). stderr: ${run.logs.stderr}\n${run.errorSnippet || ''}`,
    );
  };

  if (faucetPrivateKey && faucetPrivateKey !== primaryPrivateKey) {
    console.log(`Prefunding primary key (${short(primaryPrivateKey)}) from faucet...`);
    const prefundPrimaryRun = runZkbenchCase({
      caseName: 'prefund-primary',
      bundleDir,
      bundleStamp,
      baseEnv,
      envOverrides: {
        PRIVATE_KEY: primaryPrivateKey,
        REFUEL_PRIVATE_KEY: faucetPrivateKey,
        GAS_SAFETY_BPS: '10000',
        GAS_RESERVE: '1',
        REFUEL_AMOUNT: String(args.prefundAmount),
        PREFUND_ONLY: 'true',
        PROOF_MODE: 'synthetic',
        BATCH_SIZES: '1',
        WINDOWS_PER_SIZE: '1',
        BATCH_WINDOW_MS: '1000',
        PROOF_DEADLINE_MS: '2000',
      },
      runner: args.runner,
      dockerImage: args.dockerImage,
      goModCacheDir,
      goBuildCacheDir,
      pkHostPath,
      zkbenchExec,
    });
    ensureCaseSuccess('prefund primary', prefundPrimaryRun);
  }

  if (!args.skipBackupTakeover && !args.skipPrefundBackup) {
    const refuelSourceKey = faucetPrivateKey || primaryPrivateKey;
    if (refuelSourceKey === backupPrivateKey) {
      throw new Error('backup prefund source key must differ from backup private key');
    }
    console.log(`Prefunding backup key (${short(backupPrivateKey)}) before evidence checks...`);
    const prefundBackupRun = runZkbenchCase({
      caseName: 'prefund-backup',
      bundleDir,
      bundleStamp,
      baseEnv,
      envOverrides: {
        PRIVATE_KEY: backupPrivateKey,
        REFUEL_PRIVATE_KEY: refuelSourceKey,
        GAS_SAFETY_BPS: '10000',
        GAS_RESERVE: '1',
        REFUEL_AMOUNT: String(args.prefundAmount),
        PREFUND_ONLY: 'true',
        PROOF_MODE: 'synthetic',
        BATCH_SIZES: '1',
        WINDOWS_PER_SIZE: '1',
        BATCH_WINDOW_MS: '1000',
        PROOF_DEADLINE_MS: '2000',
      },
      runner: args.runner,
      dockerImage: args.dockerImage,
      goModCacheDir,
      goBuildCacheDir,
      pkHostPath,
      zkbenchExec,
    });
    ensureCaseSuccess('prefund backup', prefundBackupRun);
  }

  const checks = [];

  console.log('Running shielded smoke...');
  const shieldedRun = runZkbenchCase({
    caseName: 'shielded-smoke',
    bundleDir,
    bundleStamp,
    baseEnv,
    envOverrides: {
      PROOF_MODE: 'groth16',
      PROOF_CIRCUIT_ID: 'shielded-ledger-v1',
      GROTH16_PK_PATH: pkHostPath,
      BATCH_SIZES: String(args.batchSize),
      WINDOWS_PER_SIZE: String(args.windowsPerSize),
      BATCH_WINDOW_MS: '5000',
      PROOF_DEADLINE_MS: '10000',
    },
    runner: args.runner,
    dockerImage: args.dockerImage,
    goModCacheDir,
    goBuildCacheDir,
    pkHostPath,
    zkbenchExec,
  });
  const shieldedEval = evaluateShieldedSmoke(shieldedRun);
  checks.push({
    id: 'shielded-smoke',
    required: true,
    passed: shieldedEval.pass,
    attempts: [{ run: shieldedRun, evaluation: shieldedEval }],
  });

  if (!args.skipBackupTakeover) {
    const takeoverAttempts = [];

    console.log('Running backup takeover drill (primary prover rejection)...');
    const primaryFailRun = runZkbenchCase({
      caseName: 'backup-takeover-primary-fails',
      bundleDir,
      bundleStamp,
      baseEnv,
      envOverrides: {
        PRIVATE_KEY: primaryPrivateKey,
        PROOF_CONFIG_PRIVATE_KEY: proofConfigPrivateKey,
        PROVER_AUTHORITY_PRIVATE_KEY: backupPrivateKey,
        PROOF_MODE: 'groth16',
        PROOF_CIRCUIT_ID: 'shielded-ledger-v1',
        GROTH16_PK_PATH: pkHostPath,
        BATCH_SIZES: String(args.batchSize),
        WINDOWS_PER_SIZE: String(args.windowsPerSize),
        BATCH_WINDOW_MS: '5000',
        PROOF_DEADLINE_MS: '10000',
      },
      runner: args.runner,
      dockerImage: args.dockerImage,
      goModCacheDir,
      goBuildCacheDir,
      pkHostPath,
      zkbenchExec,
    });
    const primaryFailEval = evaluateBackupPrimaryFailure(primaryFailRun);
    takeoverAttempts.push({ run: primaryFailRun, evaluation: primaryFailEval });

    console.log('Running backup takeover drill (backup prover recovery)...');
    const backupRecoverRun = runZkbenchCase({
      caseName: 'backup-takeover-backup-recovers',
      bundleDir,
      bundleStamp,
      baseEnv,
      envOverrides: {
        PRIVATE_KEY: backupPrivateKey,
        REFUEL_PRIVATE_KEY: primaryPrivateKey,
        PROOF_CONFIG_PRIVATE_KEY: proofConfigPrivateKey,
        PROVER_AUTHORITY_PRIVATE_KEY: backupPrivateKey,
        GAS_SAFETY_BPS: '10000',
        GAS_RESERVE: '1',
        PROOF_MODE: 'groth16',
        PROOF_CIRCUIT_ID: 'shielded-ledger-v1',
        GROTH16_PK_PATH: pkHostPath,
        BATCH_SIZES: String(args.batchSize),
        WINDOWS_PER_SIZE: String(args.windowsPerSize),
        BATCH_WINDOW_MS: '5000',
        PROOF_DEADLINE_MS: '10000',
      },
      runner: args.runner,
      dockerImage: args.dockerImage,
      goModCacheDir,
      goBuildCacheDir,
      pkHostPath,
      zkbenchExec,
    });
    const backupRecoverEval = evaluateShieldedSmoke(backupRecoverRun);
    takeoverAttempts.push({ run: backupRecoverRun, evaluation: backupRecoverEval });

    checks.push({
      id: 'backup-takeover',
      required: true,
      passed: primaryFailEval.pass && backupRecoverEval.pass,
      attempts: takeoverAttempts,
    });
  }

  if (!args.skipNegative) {
    console.log('Running synthetic negative...');
    const negativeRun = runZkbenchCase({
      caseName: 'synthetic-negative',
      bundleDir,
      bundleStamp,
      baseEnv,
      envOverrides: {
        PROOF_MODE: 'synthetic',
        PROOF_CIRCUIT_ID: 'shielded-ledger-v1',
        BATCH_SIZES: String(args.batchSize),
        WINDOWS_PER_SIZE: String(args.windowsPerSize),
        BATCH_WINDOW_MS: '5000',
        PROOF_DEADLINE_MS: '10000',
      },
      runner: args.runner,
      dockerImage: args.dockerImage,
      goModCacheDir,
      goBuildCacheDir,
      pkHostPath,
      zkbenchExec,
    });
    const negativeEval = evaluateSyntheticNegative(negativeRun);
    checks.push({
      id: 'synthetic-negative',
      required: true,
      passed: negativeEval.pass,
      attempts: [{ run: negativeRun, evaluation: negativeEval }],
    });
  }

  if (!args.skipMalformed) {
    console.log('Running malformed-proof drill...');
    const malformedRun = runZkbenchCase({
      caseName: 'malformed-proof',
      bundleDir,
      bundleStamp,
      baseEnv,
      envOverrides: {
        PROOF_MODE: 'groth16',
        PROOF_CIRCUIT_ID: 'shielded-ledger-v1',
        GROTH16_PK_PATH: pkHostPath,
        PROOF_TAMPER_MODE: 'truncate',
        BATCH_SIZES: String(args.batchSize),
        WINDOWS_PER_SIZE: String(args.windowsPerSize),
        BATCH_WINDOW_MS: '5000',
        PROOF_DEADLINE_MS: '10000',
      },
      runner: args.runner,
      dockerImage: args.dockerImage,
      goModCacheDir,
      goBuildCacheDir,
      pkHostPath,
      zkbenchExec,
    });
    const malformedEval = evaluateMalformedProof(malformedRun);
    checks.push({
      id: 'malformed-proof',
      required: true,
      passed: malformedEval.pass,
      attempts: [{ run: malformedRun, evaluation: malformedEval }],
    });
  }

  if (!args.skipTimeout) {
    const timeoutAttempts = [];
    let timeoutPassed = false;
    for (const batch of args.timeoutBatches) {
      const caseName = `timeout-drill-b${batch}`;
      console.log(`Running timeout drill attempt (${caseName})...`);
      const timeoutRun = runZkbenchCase({
        caseName,
        bundleDir,
        bundleStamp,
        baseEnv,
        envOverrides: {
          PROOF_MODE: 'groth16',
          PROOF_CIRCUIT_ID: 'shielded-ledger-v1',
          GROTH16_PK_PATH: pkHostPath,
          BATCH_SIZES: String(batch),
          WINDOWS_PER_SIZE: '1',
          BATCH_WINDOW_MS: '1',
          PROOF_DEADLINE_MS: '1',
          PROOF_SUBMIT_DELAY_MS: '1500',
          TIMEOUT_MINUTES: String(Math.min(args.timeoutMinutes, 10)),
        },
        runner: args.runner,
        dockerImage: args.dockerImage,
        goModCacheDir,
        goBuildCacheDir,
        pkHostPath,
        zkbenchExec,
      });
      const timeoutEval = evaluateTimeoutDrill(timeoutRun);
      timeoutAttempts.push({ run: timeoutRun, evaluation: timeoutEval });
      if (timeoutEval.pass) {
        timeoutPassed = true;
        break;
      }
    }
    checks.push({
      id: 'timeout-drill',
      required: true,
      passed: timeoutPassed,
      attempts: timeoutAttempts,
    });
  }

  const overallPass = checks.every((check) => !check.required || check.passed);
  const bundleJsonPath = resolve(bundleDir, 'bundle.json');
  const bundleMdPath = resolve(bundleDir, 'bundle.md');
  const payload = {
    generatedAt: new Date().toISOString(),
    nodeUrl: args.nodeUrl,
    chainId,
    chainDiscoveryStrategy,
    runner: args.runner,
    dockerImage: args.runner === 'docker' ? args.dockerImage : '',
    zkbenchExec,
    pkPath: pkHostPath,
    discoveredChains,
    overallPass,
    checks,
  };

  writeFileSync(bundleJsonPath, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');
  writeBundleMarkdown(bundleMdPath, payload);

  console.log('');
  console.log(`Bundle JSON: ${bundleJsonPath}`);
  console.log(`Bundle MD:   ${bundleMdPath}`);
  console.log(`Verdict:     ${overallPass ? 'PASS' : 'FAIL'}`);

  if (!overallPass) {
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(`EVIDENCE BUNDLE FAILED: ${err.message || err}`);
  process.exit(1);
});
