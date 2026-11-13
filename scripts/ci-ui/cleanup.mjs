// Independent always() cleanup also handles an interrupted capture process.
import assert from 'node:assert/strict';
import { readFile, readlink, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';

assert.equal(process.env.CI, 'true');
assert.ok(process.env.RUNNER_TEMP);
const root = path.join(process.env.RUNNER_TEMP, 'pr880-fixtures');
const results = [];
let failed = false;
for (const group of ['desktop', 'narrow-de']) {
  const directory = path.join(root, group);
  try {
    const server = JSON.parse(await readFile(path.join(directory, 'server.json'), 'utf8'));
    assert.ok(Number.isSafeInteger(server.pid) && server.pid > 1);
    assert.ok(['pr880-before', 'pr880-after'].some(name => server.binary === path.join(process.env.RUNNER_TEMP, name)));
    assert.equal(server.cwd, path.join(directory, 'home'));
    try {
      // Never signal a reused PID belonging to a different process or directory.
      const argv = (await readFile(`/proc/${server.pid}/cmdline`, 'utf8')).split('\0');
      assert.equal(argv[0], server.binary);
      assert.equal(argv[1], 'ui');
      assert.equal(await readlink(`/proc/${server.pid}/cwd`), server.cwd);
      process.kill(server.pid, 'SIGTERM');
      let alive = true;
      for (let i = 0; i < 30; i++) {
        try { process.kill(server.pid, 0); } catch (error) { if (error.code !== 'ESRCH') throw error; alive = false; break; }
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      if (alive) {
        // Recheck identity before the final signal.
        assert.equal(await readlink(`/proc/${server.pid}/cwd`), server.cwd);
        assert.equal((await readFile(`/proc/${server.pid}/cmdline`, 'utf8')).split('\0')[0], server.binary);
        process.kill(server.pid, 'SIGKILL');
      }
      results.push(`${group}: stopped owned backend ${server.pid}`);
    } catch (error) {
      if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error;
      results.push(`${group}: backend already stopped`);
    }
    await rm(directory, { recursive: true, force: true });
  } catch (error) {
    if (error.code === 'ENOENT') {
      // A failure before backend launch can leave a partial fixture without a PID.
      await rm(directory, { recursive: true, force: true });
      results.push(`${group}: no backend; partial fixture removed`);
    } else {
      failed = true;
      results.push(`${group}: cleanup refused: ${error.stack}`);
    }
  }
}
if (!failed) await rm(root, { recursive: true, force: true });
await writeFile(path.join(process.env.GITHUB_WORKSPACE, 'evidence/hosted-cleanup.json'), JSON.stringify({ failed, results }, null, 2));
if (failed) process.exitCode = 1;
