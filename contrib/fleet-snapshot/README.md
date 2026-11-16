# Fleet snapshot companion (standalone proof of concept)

A standalone companion experiment, not part of Skillshare. It answers one
question for someone who runs Skillshare on several machines: which skills
each machine says it has applied, and how old that claim is. Nothing here is
built, imported or called by Skillshare, and it is not proposed for merging
into Skillshare.

There are two independent programs, written separately and held to one
contract by a shared test suite:

- `exporter/fleet_snapshot.py` runs on one machine. It reads a single,
  explicitly supplied description of that machine's targets and sync
  evidence, and writes one redacted snapshot.
- `consumer/consumer.py` runs anywhere. It reads explicitly supplied
  snapshots and an alias registry, and prints a fleet report.

Neither program discovers files, reads Skillshare configuration, opens a
network connection, starts a process, or needs credentials. Every fixture is
invented. Python 3.11 or newer, standard library only.

```sh
cd contrib/fleet-snapshot
python3 -I -B -m unittest discover -s exporter/tests
python3 -I -B -m unittest discover -s consumer
python3 -I -B -m unittest discover -s tests          # exporter and consumer together

python3 -I -B exporter/fleet_snapshot.py exporter/fixtures/synthetic-machine.input.json --out /tmp/snapshot.json
python3 -I -B consumer/consumer.py --labels consumer/fixtures/exporter-labels.json \
  --now 2026-11-16T13:00:00Z /tmp/snapshot.json
```

The report shows `m-0a1b2c3d` fresh with ten entries (five applied, three
configured, two unknown) and `spoke-missing` missing.

## Snapshot contract (schemaVersion 1)

```json
{"schemaVersion": 1, "machineId": "m-0a1b2c3d", "generatedAt": "2026-11-16T12:00:00Z",
 "entries": [{"skillId": "skill-a", "projectLabel": "alpha",
              "harnesses": ["claude"], "state": "configured"}]}
```

- Exactly these keys. `projectLabel` is omitted when there is none, never
  null or empty. `schemaVersion` is the integer 1.
- Every label (machine, skill, project, harness) is an alias matching
  `[A-Za-z][A-Za-z0-9_-]{0,63}`: no dots, spaces, slashes, `@` or leading
  digits, so a path, hostname, URL or email address cannot pass as one. A
  label that fits the grammar can still be a real name; the caller chooses
  invented or reviewed aliases. The exporter also refuses values that look
  like credentials (known token prefixes, or 40 or more token characters).
- `harnesses` holds 1 to 32 unique aliases, sorted. A snapshot holds at most
  10000 entries.
- An entry's identity is its skill, project label and harness group. Two
  entries with the same identity are refused by both programs; the exporter
  refuses to merge two physical targets that would produce them.
- `state` is `applied`, `configured` or `unknown`. It is the machine's own
  recorded, unsigned claim, not proof that an apply succeeded.
- `generatedAt` is UTC with a trailing `Z`. The exporter writes whole seconds;
  the consumer also reads up to six fractional digits.

### Freshness and clock skew

Both programs share one rule. Age is the reader's clock minus `generatedAt`.

| Age | Status |
|---|---|
| more than 300 seconds in the future | `unknown`: the clocks disagree, so freshness cannot be judged |
| from 300 seconds in the future up to the maximum age (default 86400 s) | `fresh` |
| over the maximum age | `stale` |

`unknown` and `stale` machines keep their recorded entries, which must not be
shown as current. The exporter offers the same rule as `staleness()`.

## Exporter

The input file is the machine's own view: each target's harness, directory,
sync mode and desired skills; project roots with the label to publish; and
per-directory evidence (manifest status and content, links the caller
verified). Directories and project roots are used only to group harnesses
that share a directory, and never appear in the output.

| Mode | applied | configured | unknown |
|---|---|---|---|
| merge | link verified by the caller (`verifiedLinks`) | desired, manifest absent or does not list it | manifest `symlink` without a verified link, checksum entry, unreadable manifest, no evidence |
| copy | manifest sha256 entry | desired, manifest absent or does not list it | manifest `symlink` value, unreadable manifest, no evidence |
| symlink | `dirLinkVerified` | not used | directory link not verified, no evidence |

Desired configuration alone never produces `applied`. A skill that evidence
shows is applied but is no longer desired is still reported as `applied`.

Export is opt-in per machine (`exportEnabled`) and per project: a project is
exported only when the input gives it a label, and unlisted projects are left
out entirely. Unknown fields, wrong types, unsafe labels, non-UTC timestamps,
relative directories, unexpected manifest values, conflicting modes on one
directory, and orphaned or duplicate evidence reject the whole export.
Skillshare's `ReadManifest` treats a corrupt manifest as empty; the exporter
rejects it instead.

Before writing, a redaction guard checks every output value against every
input path and project root, and this machine's hostname, username and home
directory (`--no-host-denylist` turns that part off). On any failure nothing is
written. Exit codes: 0 written, 2 unsafe or malformed input (including
duplicate JSON keys), 3 export not enabled, 4 output could not be written.

The manifest format comes from `internal/sync/manifest.go` (`managed`:
`symlink` or a sha256 checksum). The sync matrix in
`internal/server/handler_sync_matrix.go` reports `synced` from filter rules
only, so it counts as desired, not applied. Both were read at Skillshare
commit `5a6873ec`.

## Consumer

The consumer needs a caller-owned registry of every alias it may accept:

```json
{"machineIds": ["spoke-a"], "skillIds": ["skill-alpha"],
 "projectLabels": ["project-demo"], "harnesses": ["harness-a"]}
```

Each registered machine is expected. One with no snapshot is reported
`missing`, which means none was supplied, not that it is offline. The registry
is not authentication or enrollment.

The whole batch is rejected, with no partial report, for a label that is
unsafe or not registered, an unknown field, a duplicate entry, a second
snapshot for the same machine (even an identical or newer one), duplicate JSON
keys, `NaN` or `Infinity`, a file over 1 MiB, more than 256 snapshots, or a
path whose final component is a symlink, directory or FIFO. Error messages never repeat input
values or paths. Explicit paths may still pass through symlinked directories,
so this is not a sandbox for untrusted path arguments.

The consumer passes recorded states through unchanged. It never turns
`configured` into `applied`, and it never infers a state from freshness or
from a missing machine.

## Limits

- Snapshots are unsigned and carried as files; anyone who can write one can
  claim any state for any registered machine.
- `applied` is what the machine recorded, checked against local evidence the
  caller supplied. It is not an independent check of the target.
- Gathering the input is left to the caller; this experiment does not read a
  real Skillshare installation.
- Skill names are published as they are, so they must already fit the alias
  grammar. Skillshare's own flat names can start with `_` (tracked
  repositories) or a digit, or run past 64 characters for nested skills. Any
  such name in a manifest makes the exporter refuse the whole snapshot, even if
  no target wants that skill. Mapping real names to published aliases is left
  for later.
