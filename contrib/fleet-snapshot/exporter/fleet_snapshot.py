#!/usr/bin/env python3
"""Portable, redacted Skillshare fleet snapshot builder (standalone PoC).

Reads one explicit, caller-supplied description of this machine's targets and
sync evidence, and writes a portable snapshot:

    {"schemaVersion": 1, "machineId": str, "generatedAt": "YYYY-MM-DDTHH:MM:SSZ",
     "entries": [{"skillId": str, "projectLabel"?: str,
                  "harnesses": [str], "state": "configured"|"applied"|"unknown"}]}

It does not discover anything: no config is read, no target directory is
walked, no network is opened. Anything outside the expected shape fails the
whole export (fail closed); nothing is written on failure.

State rules (per skill, per target directory group):
  applied     sync evidence shows the skill in the directory
              (copy: manifest checksum; merge: manifest "symlink" entry AND a
              caller-verified link, or a verified link alone; symlink mode:
              the directory link itself was verified).
  configured  desired by config, and evidence was inspected and shows it is
              not there (manifest confirmed absent, or present without it).
  unknown     desired, but evidence is missing, unreadable, unverified or
              inconsistent with the mode.
Desired config alone never yields "applied".
"""

from __future__ import annotations

import json
import os
import posixpath
import re
import socket
import getpass
import sys
from dataclasses import dataclass
from datetime import datetime, timezone

SCHEMA_VERSION = 1
INPUT_VERSION = 1
STATES = ("configured", "applied", "unknown")
MODES = ("merge", "copy", "symlink")

MAX_SKEW_SECONDS = 300  # shared with the consumer: future-dated beyond this is unknown
MAX_ENTRIES = 10_000
MAX_HARNESSES = 32

_ISO_UTC = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\Z")
# One opt-in alias grammar for every emitted label (machine, skill, project,
# harness), identical to the consumer's. No dots, spaces, slashes or leading
# digits, so a path, hostname, URL or email address cannot pass as a label.
_ALIAS = re.compile(r"[A-Za-z][A-Za-z0-9_-]{0,63}\Z")
_SHA256 = re.compile(r"[0-9a-f]{64}\Z")
# Token shapes that must never leave the machine: a known credential prefix at
# the start of a word followed by a token body, or any long token-like run.
# A prefix inside an ordinary word ("task-runner") is not a credential.
_SECRETISH = re.compile(
    r"(?<![A-Za-z0-9])(?:ghp_|gho_|ghs_|github_pat_|glpat-|xox[abprs]-|sk-|AKIA|ASIA|AIza)"
    r"[A-Za-z0-9_-]{8,}|-----BEGIN|[A-Za-z0-9+/=_-]{40,}"
)
_MANIFEST_KEYS = {"managed", "mtimes", "sources", "naming", "updated_at"}


class SnapshotError(ValueError):
    """Input or evidence is unsafe or malformed; nothing must be written."""


class OptInRequired(SnapshotError):
    """Export is not enabled for this machine."""


# ---------------------------------------------------------------- validation

def _obj(value, where, required, optional=()):
    if not isinstance(value, dict):
        raise SnapshotError(f"{where}: expected object")
    unknown = set(value) - set(required) - set(optional)
    if unknown:
        raise SnapshotError(f"{where}: unknown field(s) {sorted(unknown)}")
    missing = [k for k in required if k not in value]
    if missing:
        raise SnapshotError(f"{where}: missing field(s) {missing}")
    return value


def _str(value, where, pattern=None):
    if not isinstance(value, str):
        raise SnapshotError(f"{where}: expected string")
    if pattern is not None and not pattern.match(value):
        raise SnapshotError(f"{where}: value not allowed")
    if pattern is not None and _SECRETISH.search(value):
        raise SnapshotError(f"{where}: value looks like a credential")
    return value


def _str_list(value, where, pattern):
    if not isinstance(value, list):
        raise SnapshotError(f"{where}: expected array")
    return [_str(v, f"{where}[{i}]", pattern) for i, v in enumerate(value)]


def _bool(value, where):
    if not isinstance(value, bool):
        raise SnapshotError(f"{where}: expected boolean")
    return value


def _iso_utc(value, where):
    _str(value, where, _ISO_UTC)
    try:
        datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ")
    except ValueError as exc:
        raise SnapshotError(f"{where}: invalid timestamp") from exc
    return value


def _local_dir(value, where):
    """A target directory: used only locally for grouping, never emitted."""
    _str(value, where)
    if "\x00" in value or not value.startswith("/"):
        raise SnapshotError(f"{where}: target dir must be an absolute path")
    norm = posixpath.normpath(value)
    if norm.startswith("//"):
        norm = "/" + norm.lstrip("/")
    return norm


def _validate_manifest(content, where):
    """Validate a .skillshare-manifest.json object. Unlike upstream
    ReadManifest (corrupt -> empty), anything unexpected fails closed."""
    _obj(content, where, required=("managed",), optional=_MANIFEST_KEYS - {"managed"})
    managed = content["managed"]
    if not isinstance(managed, dict):
        raise SnapshotError(f"{where}.managed: expected object")
    out = {}
    for name, val in managed.items():
        _str(name, f"{where}.managed key", _ALIAS)
        if not isinstance(val, str) or not (val == "symlink" or _SHA256.match(val)):
            raise SnapshotError(f"{where}.managed: expected 'symlink' or sha256 value")
        out[name] = val
    for key, check in (("mtimes", lambda v: isinstance(v, int) and not isinstance(v, bool)),
                       ("sources", lambda v: isinstance(v, str) and bool(_SHA256.match(v))),
                       ("naming", lambda v: isinstance(v, str) and len(v) <= 64)):
        if key in content:
            m = content[key]
            if not isinstance(m, dict):
                raise SnapshotError(f"{where}.{key}: expected object")
            for k, v in m.items():
                _str(k, f"{where}.{key} key", _ALIAS)
                if not check(v):
                    raise SnapshotError(f"{where}.{key}: bad value")
    if "updated_at" in content and not isinstance(content["updated_at"], str):
        raise SnapshotError(f"{where}.updated_at: expected string")
    return out


# ---------------------------------------------------------------- model

@dataclass
class _Group:
    dir: str
    mode: str
    project: str | None
    harnesses: set
    desired: set


def _parse_input(doc):
    _obj(doc, "input",
         required=("inputVersion", "exportEnabled", "machineId", "generatedAt", "targets"),
         optional=("projects", "evidence"))
    if doc["inputVersion"] != INPUT_VERSION or isinstance(doc["inputVersion"], bool):
        raise SnapshotError("input.inputVersion: unsupported")
    if _bool(doc["exportEnabled"], "input.exportEnabled") is not True:
        raise OptInRequired("export is not enabled for this machine")
    machine_id = _str(doc["machineId"], "input.machineId", _ALIAS)
    generated_at = _iso_utc(doc["generatedAt"], "input.generatedAt")

    projects = doc.get("projects", {})
    if not isinstance(projects, dict):
        raise SnapshotError("input.projects: expected object")
    labels = {}
    for n, (key, label) in enumerate(sorted(projects.items())):
        root = _local_dir(key, f"input.projects key #{n}")
        if root in labels:
            raise SnapshotError(f"input.projects key #{n}: same project listed twice")
        labels[root] = _str(label, f"input.projects value #{n}", _ALIAS)
    if len(set(labels.values())) != len(labels):
        raise SnapshotError("input.projects: labels must be unique")

    if not isinstance(doc["targets"], list):
        raise SnapshotError("input.targets: expected array")
    groups: dict[str, _Group] = {}
    skipped = 0
    seen_harness_dir = set()
    for i, t in enumerate(doc["targets"]):
        w = f"input.targets[{i}]"
        _obj(t, w, required=("harness", "dir", "mode", "desired"), optional=("project",))
        harness = _str(t["harness"], f"{w}.harness", _ALIAS)
        d = _local_dir(t["dir"], f"{w}.dir")
        mode = _str(t["mode"], f"{w}.mode")
        if mode not in MODES:
            raise SnapshotError(f"{w}.mode: value not allowed")
        desired = set(_str_list(t["desired"], f"{w}.desired", _ALIAS))
        project = t.get("project")
        if project is not None:
            project = _local_dir(project, f"{w}.project")
            if project not in labels:
                skipped += 1  # project not opted in: export nothing about it
                continue
        if (harness, d) in seen_harness_dir:
            raise SnapshotError(f"{w}: duplicate harness for the same dir")
        seen_harness_dir.add((harness, d))
        g = groups.get(d)
        if g is None:
            groups[d] = _Group(d, mode, project, {harness}, desired)
            continue
        # Same directory shared by several harnesses: one physical target.
        if g.mode != mode:
            raise SnapshotError(f"{w}: harnesses sharing a dir disagree on mode")
        if g.project != project:
            raise SnapshotError(f"{w}: dir is shared across scopes/projects")
        g.harnesses.add(harness)
        g.desired |= desired

    evidence = {}
    ev_list = doc.get("evidence", [])
    if not isinstance(ev_list, list):
        raise SnapshotError("input.evidence: expected array")
    all_dirs = {_local_dir(t["dir"], "dir") for t in doc["targets"]}
    for i, e in enumerate(ev_list):
        w = f"input.evidence[{i}]"
        _obj(e, w, required=("dir", "manifest"), optional=("verifiedLinks", "dirLinkVerified"))
        d = _local_dir(e["dir"], f"{w}.dir")
        if d in evidence:
            raise SnapshotError(f"{w}: duplicate evidence for dir")
        if d not in all_dirs:
            raise SnapshotError(f"{w}: evidence for a dir with no target")
        m = _obj(e["manifest"], f"{w}.manifest", required=("status",), optional=("content",))
        status = m["status"]
        if status == "present":
            if "content" not in m:
                raise SnapshotError(f"{w}.manifest: present without content")
            managed = _validate_manifest(m["content"], f"{w}.manifest.content")
        elif status in ("absent", "unreadable"):
            if "content" in m:
                raise SnapshotError(f"{w}.manifest: content not allowed for {status}")
            managed = None
        else:
            raise SnapshotError(f"{w}.manifest.status: value not allowed")
        evidence[d] = {
            "manifest": status,
            "managed": managed,
            "links": set(_str_list(e.get("verifiedLinks", []), f"{w}.verifiedLinks", _ALIAS)),
            "dirLink": _bool(e.get("dirLinkVerified", False), f"{w}.dirLinkVerified"),
        }
    return machine_id, generated_at, labels, groups, evidence, skipped


def _state(skill, mode, desired, ev):
    """Return the state for one skill in one group, or None to omit it."""
    if ev is None:
        return "unknown" if desired else None
    managed, links = ev["managed"], ev["links"]
    if mode == "symlink":
        # Whole directory is a link to the source; no per-skill manifest.
        if not desired:
            return None
        return "applied" if ev["dirLink"] else "unknown"
    if ev["manifest"] == "unreadable":
        if mode == "merge" and skill in links:
            return "applied"
        return "unknown" if desired else None
    rec = managed.get(skill) if managed is not None else None
    if mode == "copy":
        if rec is not None and rec != "symlink":
            return "applied"
        if rec == "symlink":
            return "unknown"  # manifest disagrees with the configured mode
        return "configured" if desired else None
    # merge
    if skill in links:
        return "applied"
    if rec == "symlink":
        return "unknown"  # recorded, but the link was not verified
    if rec is not None:
        return "unknown"  # checksum entry under merge: inconsistent
    return "configured" if desired else None


def build_snapshot(doc, denylist=()):
    """Build a validated, redacted, deterministic snapshot dict."""
    machine_id, generated_at, labels, groups, evidence, _ = _parse_input(doc)
    rows = []
    for d in sorted(groups):
        g = groups[d]
        ev = evidence.get(d)
        candidates = set(g.desired)
        if ev is not None:
            if ev["managed"]:
                candidates |= set(ev["managed"])
            candidates |= ev["links"]
        for skill in candidates:
            st = _state(skill, g.mode, skill in g.desired, ev)
            if st is None:
                continue
            label = labels[g.project] if g.project is not None else None
            rows.append((skill, label or "", tuple(sorted(g.harnesses)), st))
    entries = []
    for skill, label, harnesses, st in sorted(rows):
        e = {"skillId": skill}
        if label:
            e["projectLabel"] = label
        e["harnesses"] = list(harnesses)
        e["state"] = st
        entries.append(e)
    snap = {"schemaVersion": SCHEMA_VERSION, "machineId": machine_id,
            "generatedAt": generated_at, "entries": entries}
    validate_snapshot(snap)
    _redaction_guard(snap, doc, denylist)
    return snap


def _redaction_guard(snap, doc, denylist):
    """Fail closed if any local-only string reached the serialized output."""
    text = serialize(snap)
    secrets = set(denylist)
    for t in doc.get("targets", []):
        secrets.add(t["dir"])
        secrets.add(_local_dir(t["dir"], "dir"))
        if t.get("project"):
            secrets.add(t["project"])
    for e in doc.get("evidence", []):
        secrets.add(e["dir"])
    for key in doc.get("projects", {}):
        secrets.add(key)
    secrets = {s for s in secrets if isinstance(s, str) and len(s) >= 3}
    text = text.lower()
    values = [v.lower() for v in _emitted_strings(snap)]
    for s in secrets:
        sl = s.lower()
        # Long or path-like values match anywhere; short ones only as whole
        # words, so a host named "dex" does not trip on the harness "codex".
        if (len(s) >= 8 or "/" in s) and sl in text:
            raise SnapshotError("redaction guard: a local-only value reached the output")
        word = re.compile(r"(?<![a-z0-9])" + re.escape(sl) + r"(?![a-z0-9])")
        if any(word.search(v) for v in values):
            raise SnapshotError("redaction guard: a local-only value reached the output")


def _emitted_strings(snap):
    out = [snap["machineId"]]
    for e in snap["entries"]:
        out.append(e["skillId"])
        out.extend(e["harnesses"])
        if "projectLabel" in e:
            out.append(e["projectLabel"])
    return out


def default_denylist():
    """Identity strings of the exporting machine; read, never emitted."""
    out = []
    for fn in (socket.gethostname, getpass.getuser):
        try:
            v = fn()
        except Exception:
            continue
        if v:
            out.append(v)
            out.append(v.split(".")[0])
    home = os.path.expanduser("~")
    if home and home != "~":
        out.append(home)
    return out


# ---------------------------------------------------------------- contract

def validate_snapshot(snap):
    """Validate the portable snapshot contract (schemaVersion 1)."""
    _obj(snap, "snapshot", required=("schemaVersion", "machineId", "generatedAt", "entries"))
    if snap["schemaVersion"] != SCHEMA_VERSION or isinstance(snap["schemaVersion"], bool):
        raise SnapshotError("snapshot.schemaVersion: unsupported")
    _str(snap["machineId"], "snapshot.machineId", _ALIAS)
    _iso_utc(snap["generatedAt"], "snapshot.generatedAt")
    if not isinstance(snap["entries"], list) or len(snap["entries"]) > MAX_ENTRIES:
        raise SnapshotError("snapshot.entries: expected array of at most 10000")
    seen = set()
    for i, e in enumerate(snap["entries"]):
        w = f"snapshot.entries[{i}]"
        _obj(e, w, required=("skillId", "harnesses", "state"), optional=("projectLabel",))
        _str(e["skillId"], f"{w}.skillId", _ALIAS)
        if "projectLabel" in e:
            _str(e["projectLabel"], f"{w}.projectLabel", _ALIAS)
        h = _str_list(e["harnesses"], f"{w}.harnesses", _ALIAS)
        if not h or len(h) > MAX_HARNESSES or h != sorted(set(h)):
            raise SnapshotError(f"{w}.harnesses: must be 1-32 sorted, unique values")
        if e["state"] not in STATES:
            raise SnapshotError(f"{w}.state: value not allowed")
        # Identity matches the consumer: two physical targets with the same
        # harness group and scope cannot be told apart, so refuse them.
        identity = (e["skillId"], e.get("projectLabel"), tuple(h))
        if identity in seen:
            raise SnapshotError(f"{w}: duplicate skill/project/harness group")
        seen.add(identity)
    return snap


def staleness(snap, now, max_age_seconds, max_skew_seconds=MAX_SKEW_SECONDS):
    """'fresh' | 'stale' | 'unknown' (future-dated beyond allowed clock skew)."""
    validate_snapshot(snap)
    ts = datetime.strptime(snap["generatedAt"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    age = (now - ts).total_seconds()
    if age < -max_skew_seconds:
        return "unknown"
    return "fresh" if age <= max_age_seconds else "stale"


def serialize(snap):
    return json.dumps(snap, sort_keys=True, indent=2, ensure_ascii=True) + "\n"


# ---------------------------------------------------------------- CLI

def _write_atomic(path, data):
    tmp = f"{path}.tmp-{os.getpid()}"
    try:
        with open(tmp, "x", encoding="utf-8") as f:
            f.write(data)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, path)
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise


def _unique_object(pairs):
    out = {}
    for key, value in pairs:
        if key in out:
            raise SnapshotError("input: duplicate JSON key")
        out[key] = value
    return out


def _no_constant(_):
    raise SnapshotError("input: NaN and Infinity are not allowed")


def main(argv=None):
    import argparse
    p = argparse.ArgumentParser(description="Build a redacted fleet snapshot from explicit input.")
    p.add_argument("input", help="explicit synthetic input JSON")
    p.add_argument("--out", help="write snapshot here (atomic); default stdout")
    p.add_argument("--no-host-denylist", action="store_true",
                   help="do not add this machine's hostname/user/home to the redaction guard")
    a = p.parse_args(argv)
    deny = () if a.no_host_denylist else default_denylist()
    try:
        try:
            with open(a.input, encoding="utf-8") as f:
                doc = json.load(f, object_pairs_hook=_unique_object, parse_constant=_no_constant)
        except (OSError, ValueError, RecursionError) as exc:
            if isinstance(exc, SnapshotError):
                raise
            print(f"error: cannot read input: {exc.__class__.__name__}", file=sys.stderr)
            return 2
        snap = build_snapshot(doc, denylist=deny)
    except OptInRequired as exc:
        print(f"refused: {exc}", file=sys.stderr)
        return 3
    except SnapshotError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    data = serialize(snap)
    if a.out:
        try:
            _write_atomic(a.out, data)
        except OSError as exc:
            print(f"error: cannot write output: {exc.__class__.__name__}", file=sys.stderr)
            return 4
    else:
        sys.stdout.write(data)
    return 0


if __name__ == "__main__":
    sys.exit(main())
