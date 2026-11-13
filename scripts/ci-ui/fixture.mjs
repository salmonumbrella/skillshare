// Hosted CI only. No inherited account, native-client, Git or product config.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { cp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';

export const pins = {
  before: 'b7c34d6af5ebf881ee2cce70023d33bddce80964',
  after: '2c27e52c8e7ed9ddca638771dc9ebbcb55c3f05d',
};

export function command(file, args, options = {}) {
  return execFileSync(file, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], ...options });
}

export async function makeFixture(root, app, binary, names) {
  await mkdir(root, { recursive: true });
  const home = path.join(root, 'home');
  const source = path.join(home, 'skills');
  const env = {
    PATH: process.env.PATH, LANG: 'C.UTF-8', TZ: 'UTC',
    HOME: home, USERPROFILE: home, USER: 'fixture', LOGNAME: 'fixture',
    XDG_CONFIG_HOME: path.join(home, '.config'), XDG_CACHE_HOME: path.join(home, '.cache'),
    XDG_DATA_HOME: path.join(home, '.local/share'), XDG_STATE_HOME: path.join(home, '.local/state'),
    XDG_RUNTIME_DIR: path.join(home, 'run'), TMPDIR: path.join(home, 'tmp'),
    APPDATA: path.join(home, 'AppData/Roaming'), LOCALAPPDATA: path.join(home, 'AppData/Local'),
    CODEX_HOME: path.join(home, 'native/codex'), CLAUDE_CONFIG_DIR: path.join(home, 'native/claude'),
    COPILOT_HOME: path.join(home, 'native/copilot'), OPENCODE_CONFIG_DIR: path.join(home, 'native/opencode'),
    PI_CODING_AGENT_DIR: path.join(home, 'native/pi'),
    SKILLSHARE_CONFIG: path.join(home, '.config/skillshare/config.yaml'),
    SKILLSHARE_AUTO_UPDATE_CHECK: 'false',
    GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: path.join(home, 'gitconfig'),
    GIT_TERMINAL_PROMPT: '0', GIT_AUTHOR_NAME: 'Fixture', GIT_COMMITTER_NAME: 'Fixture',
    GIT_AUTHOR_EMAIL: 'fixture@example.com', GIT_COMMITTER_EMAIL: 'fixture@example.com',
    GIT_AUTHOR_DATE: '2026-01-01T00:00:00Z', GIT_COMMITTER_DATE: '2026-01-01T00:00:00Z',
  };
  await Promise.all(Object.entries(env).filter(([key]) =>
    /^(HOME|USERPROFILE|XDG_|APPDATA|LOCALAPPDATA|CODEX_HOME|CLAUDE_CONFIG_DIR|COPILOT_HOME|OPENCODE_CONFIG_DIR|PI_CODING_AGENT_DIR|TMPDIR)/.test(key)
  ).map(([, dir]) => mkdir(dir, { recursive: true })));
  await mkdir(source, { recursive: true });
  await mkdir(path.dirname(env.SKILLSHARE_CONFIG), { recursive: true });
  await writeFile(env.GIT_CONFIG_GLOBAL, '[init]\n\tdefaultBranch = main\n[protocol "file"]\n\tallow = always\n');
  await writeFile(env.SKILLSHARE_CONFIG,
    `sources:\n  skills: ${JSON.stringify(source)}\nmode: merge\ntargets: {}\nupdates:\n  auto_check: false\n`);
  // A real local Git remote; no network source and no real account.
  const origin = path.join(root, 'origin');
  await mkdir(path.join(origin, 'sample'), { recursive: true });
  await writeFile(path.join(origin, 'sample/SKILL.md'), '---\nname: sample\ndescription: Synthetic CI skill\n---\nFixture only.\n');
  command('git', ['init', origin], { env, cwd: home });
  command('git', ['-C', origin, 'add', 'sample/SKILL.md'], { env });
  command('git', ['-C', origin, 'commit', '-m', 'Add synthetic skill'], { env });
  const repoURL = `file://${origin}`;
  const entries = Object.fromEntries(names.map(name => [name, {
    source: repoURL, repo_url: repoURL, type: 'github', tracked: true, branch: 'main',
  }]));
  const metadata = path.join(source, '.metadata.json');
  await writeFile(metadata, JSON.stringify({ version: 1, entries }, null, 2));
  await mkdir(path.join(source, '_occupied'), { recursive: true });
  const occupant = path.join(source, '_occupied/keep.txt');
  await writeFile(occupant, 'Synthetic occupant must stay byte-identical.\n');
  const ignore = path.join(source, '.gitignore');
  await writeFile(ignore, names.map(name => `${name}/`).join('\n') + '\n');
  async function restore(name, dirty = false) {
    command('git', ['clone', repoURL, path.join(source, name)], { env, cwd: home });
    if (dirty) await writeFile(path.join(source, name, 'sample/SKILL.md'), 'Synthetic uncommitted edit.\n', { flag: 'a' });
  }
  await restore('_clean');
  await restore('_dirty', true);
  const cache = path.join(env.XDG_CACHE_HOME, 'skillshare/ui/dev');
  await mkdir(cache, { recursive: true });
  await cp(path.join(app, 'ui/dist'), cache, { recursive: true });
  let server;
  let serverDone;
  let spawnError;
  const log = [];
  async function start(port) {
    server = spawn(binary, ['ui', '-g', '--no-open', '--host', '127.0.0.1', '--port', String(port)], {
      env, cwd: home, stdio: ['ignore', 'pipe', 'pipe'],
    });
    serverDone = new Promise(resolve => server.once('exit', resolve));
    server.on('error', error => { spawnError = error; });
    await writeFile(path.join(root, 'server.json'), JSON.stringify({ pid: server.pid, binary, cwd: home }));
    server.stdout.on('data', data => log.push(data.toString()));
    server.stderr.on('data', data => log.push(data.toString()));
    const url = `http://127.0.0.1:${port}`;
    for (let i = 0; i < 200; i++) {
      if (spawnError) throw spawnError;
      if (server.exitCode !== null) throw new Error(`Backend exited: ${log.join('')}`);
      try { if ((await fetch(`${url}/api/health`, { signal: AbortSignal.timeout(1000) })).ok) return url; } catch { /* wait for listener */ }
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    throw new Error(`Backend failed to become healthy: ${log.join('')}`);
  }
  async function stop() {
    if (server?.pid && !spawnError && server.exitCode === null && server.signalCode === null) {
      server.kill('SIGTERM');
      const timer = setTimeout(() => server.kill('SIGKILL'), 3000);
      try { await serverDone; } finally { clearTimeout(timer); }
    }
  }
  return {
    source, metadata, occupant, ignore, restore, start, stop, log,
    async assertRecordRemoved(name) {
      const store = JSON.parse(await readFile(metadata, 'utf8'));
      assert.equal(store.entries[name], undefined);
    },
    async cleanup() { await stop(); await rm(root, { recursive: true, force: true }); },
  };
}
