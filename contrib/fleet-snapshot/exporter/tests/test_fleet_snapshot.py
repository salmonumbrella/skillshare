"""Unit and smoke tests for the snapshot builder. Synthetic data only."""

import copy
import json
import os
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
sys.path.insert(0, ROOT)

import fleet_snapshot as fs  # noqa: E402

FIXTURE = os.path.join(ROOT, "fixtures", "synthetic-machine.input.json")
GOLDEN = os.path.join(ROOT, "fixtures", "synthetic-machine.snapshot.golden.json")
SHA = "a" * 64


def load():
    with open(FIXTURE, encoding="utf-8") as f:
        return json.load(f)


def one(mode, desired, evidence=None, harness="claude", d="/synthetic/x/skills"):
    doc = {"inputVersion": 1, "exportEnabled": True, "machineId": "m-test0001",
           "generatedAt": "2026-11-16T12:00:00Z",
           "targets": [{"harness": harness, "dir": d, "mode": mode, "desired": desired}]}
    if evidence is not None:
        evidence = dict(evidence, dir=d)
        doc["evidence"] = [evidence]
    return doc


def states(doc):
    return {(e["skillId"], e.get("projectLabel")): e["state"]
            for e in fs.build_snapshot(doc)["entries"]}


class Determinism(unittest.TestCase):
    def test_golden(self):
        with open(GOLDEN, encoding="utf-8") as f:
            self.assertEqual(fs.serialize(fs.build_snapshot(load())), f.read())

    def test_input_order_does_not_matter(self):
        a = load()
        b = copy.deepcopy(a)
        b["targets"].reverse()
        b["evidence"].reverse()
        for t in b["targets"]:
            t["desired"].reverse()
        self.assertEqual(fs.serialize(fs.build_snapshot(a)), fs.serialize(fs.build_snapshot(b)))

    def test_exact_top_level_shape(self):
        snap = fs.build_snapshot(load())
        self.assertEqual(set(snap), {"schemaVersion", "machineId", "generatedAt", "entries"})
        for e in snap["entries"]:
            self.assertLessEqual(set(e), {"skillId", "projectLabel", "harnesses", "state"})


class Redaction(unittest.TestCase):
    def test_no_paths_or_unopted_projects(self):
        text = fs.serialize(fs.build_snapshot(load()))
        for needle in ("/synthetic", "u1", "project-one", "project-two",
                       "skill-secret-project", ".agents", "manifest", "1111", "updated_at"):
            self.assertNotIn(needle, text)

    def test_shared_dir_grouped_once(self):
        snap = fs.build_snapshot(load())
        rows = [e for e in snap["entries"] if e["skillId"] == "skill-a" and "agents" in e["harnesses"]]
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["harnesses"], ["agents", "claude"])

    def test_denylist_whole_token_fails_closed(self):
        with self.assertRaisesRegex(fs.SnapshotError, "redaction guard"):
            fs.build_snapshot(load(), denylist=["alpha"])  # equals a label

    def test_denylist_substring_of_token_is_not_a_leak(self):
        # A short name found only inside a longer token ("cursor") is not a leak.
        fs.build_snapshot(load(), denylist=["urso"])

    def test_denylist_short_hyphenated_and_long_mixed_case_values_fail(self):
        for skill, deny in (("notes-my-host", "my-host"), ("skill-work-laptop", "Work-Laptop")):
            with self.subTest(deny=deny), self.assertRaisesRegex(fs.SnapshotError, "redaction guard"):
                fs.build_snapshot(one("merge", [skill]), denylist=[deny])

    def test_denylist_long_value_substring_fails(self):
        with self.assertRaisesRegex(fs.SnapshotError, "redaction guard"):
            fs.build_snapshot(load(), denylist=["skill-orphan"])


class StateLogic(unittest.TestCase):
    def test_desired_without_evidence_is_unknown_never_applied(self):
        self.assertEqual(states(one("merge", ["s1"])), {("s1", None): "unknown"})
        self.assertEqual(states(one("copy", ["s1"])), {("s1", None): "unknown"})
        self.assertEqual(states(one("symlink", ["s1"])), {("s1", None): "unknown"})

    def test_manifest_absent_means_configured(self):
        ev = {"manifest": {"status": "absent"}}
        self.assertEqual(states(one("merge", ["s1"], ev)), {("s1", None): "configured"})

    def test_unreadable_manifest_is_unknown(self):
        ev = {"manifest": {"status": "unreadable"}}
        self.assertEqual(states(one("copy", ["s1"], ev)), {("s1", None): "unknown"})

    def test_merge_needs_verified_link(self):
        ev = {"manifest": {"status": "present", "content": {"managed": {"s1": "symlink", "s2": "symlink"}}},
              "verifiedLinks": ["s1"]}
        self.assertEqual(states(one("merge", ["s1", "s2"], ev)),
                         {("s1", None): "applied", ("s2", None): "unknown"})

    def test_copy_checksum_is_applied_symlink_value_is_inconsistent(self):
        ev = {"manifest": {"status": "present", "content": {"managed": {"s1": SHA, "s2": "symlink"}}}}
        self.assertEqual(states(one("copy", ["s1", "s2", "s3"], ev)),
                         {("s1", None): "applied", ("s2", None): "unknown", ("s3", None): "configured"})

    def test_symlink_mode_needs_dir_link(self):
        ev = {"manifest": {"status": "absent"}, "dirLinkVerified": False}
        self.assertEqual(states(one("symlink", ["s1"], ev)), {("s1", None): "unknown"})
        ev["dirLinkVerified"] = True
        self.assertEqual(states(one("symlink", ["s1"], ev)), {("s1", None): "applied"})

    def test_applied_but_not_desired_is_reported(self):
        ev = {"manifest": {"status": "present", "content": {"managed": {"old": SHA}}}}
        self.assertEqual(states(one("copy", [], ev)), {("old", None): "applied"})


class FailClosed(unittest.TestCase):
    def bad(self, doc, pattern=""):
        with self.assertRaisesRegex(fs.SnapshotError, pattern):
            fs.build_snapshot(doc)

    def test_opt_in_required(self):
        doc = load()
        doc["exportEnabled"] = False
        with self.assertRaises(fs.OptInRequired):
            fs.build_snapshot(doc)

    def test_unknown_fields_rejected(self):
        doc = load(); doc["hostname"] = "x"; self.bad(doc, "unknown field")
        doc = load(); doc["targets"][0]["path"] = "/x"; self.bad(doc, "unknown field")
        doc = load(); doc["evidence"][0]["manifest"]["content"]["extra"] = 1; self.bad(doc, "unknown field")

    def test_bad_manifest_values(self):
        for managed in ({"s1": "deadbeef"}, {"s1": 1}, {"../s1": SHA}, {"a/b": SHA}, []):
            ev = {"manifest": {"status": "present", "content": {"managed": managed}}}
            self.bad(one("copy", ["s1"], ev))

    def test_manifest_status_shape(self):
        self.bad(one("copy", [], {"manifest": {"status": "present"}}), "without content")
        self.bad(one("copy", [], {"manifest": {"status": "absent", "content": {"managed": {}}}}), "not allowed")
        self.bad(one("copy", [], {"manifest": {"status": "maybe"}}), "not allowed")

    def test_unsafe_ids_and_labels(self):
        for sid in ("https://x.example/s", "~/s", "s:1", "", "ghp_" + "x" * 10, "A" * 48):
            self.bad(one("merge", [sid]))
        doc = load(); doc["projects"] = {"/synthetic/home/u1/work/project-one": "a/b"}; self.bad(doc)
        doc = load(); doc["machineId"] = "Host.Local"; self.bad(doc)
        doc = load(); doc["machineId"] = "m-0a1b2c3d\n"; self.bad(doc)

    def test_labels_use_the_shared_alias_grammar(self):
        # Opt-in project labels, skill IDs, harnesses and machine IDs all use
        # the consumer's grammar: no spaces, dots or leading digits.
        for label in ("alpha beta", "alpha.one", "1alpha", "alpha\n", "a" * 65, "猫"):
            doc = load(); doc["projects"] = {"/synthetic/home/u1/work/project-one": label}
            self.bad(doc)
        for sid in ("skill.x", "1skill", "skill-a\n"):
            self.bad(one("merge", [sid]))
        self.bad(one("merge", ["s1"], harness="9harness"))
        doc = load(); doc["machineId"] = "0a1b2c3d"; self.bad(doc)

    def test_indistinguishable_targets_are_refused_not_merged(self):
        # Two global targets for the same harness group would produce entries
        # a reader cannot tell apart, even when their states agree.
        for second in ("absent", "present"):
            doc = one("copy", ["s1"], {"manifest": {"status": "absent"}}, d="/synthetic/a/skills")
            doc["targets"].append({"harness": "claude", "dir": "/synthetic/b/skills",
                                   "mode": "copy", "desired": ["s1"]})
            content = {"managed": {"s1": SHA}} if second == "present" else None
            ev = {"dir": "/synthetic/b/skills", "manifest": {"status": second}}
            if content:
                ev["manifest"]["content"] = content
            doc["evidence"].append(ev)
            with self.subTest(second=second):
                self.bad(doc, "duplicate")
        doc = load(); doc["generatedAt"] = "2026-11-16T12:00:00+02:00"; self.bad(doc)
        doc = load(); doc["generatedAt"] = "2026-02-30T12:00:00Z"; self.bad(doc)

    def test_credential_prefix_only_counts_at_word_start_with_a_body(self):
        for sid in ("task-runner", "desk-01", "ask-claude", "sk-lint", "MASIAN"):
            self.assertEqual(states(one("merge", [sid])), {(sid, None): "unknown"})
        for sid in ("sk-abcdefgh12", "skill-ghp_abcdefgh12", "AKIAabcdefgh12"):
            self.bad(one("merge", [sid]), "credential")

    def test_same_project_root_listed_twice_rejected(self):
        doc = load()
        doc["projects"]["/synthetic/home/u1/work/project-one/"] = "beta"
        self.bad(doc, "listed twice")

    def test_relative_dir_rejected(self):
        self.bad(one("merge", ["s1"], d="relative/skills"), "absolute")

    def test_shared_dir_mode_conflict(self):
        doc = load(); doc["targets"][1]["mode"] = "copy"; self.bad(doc, "disagree on mode")

    def test_evidence_must_match_a_target(self):
        doc = load()
        doc["evidence"].append({"dir": "/synthetic/elsewhere", "manifest": {"status": "absent"}})
        self.bad(doc, "no target")

    def test_duplicate_evidence(self):
        doc = load()
        dup = copy.deepcopy(doc["evidence"][2]); dup["dir"] += "/"
        doc["evidence"].append(dup)
        self.bad(doc, "duplicate")


class Staleness(unittest.TestCase):
    def test_fresh_stale_future(self):
        snap = fs.build_snapshot(load())
        t0 = datetime(2026, 11, 16, 12, 0, 0, tzinfo=timezone.utc)
        self.assertEqual(fs.staleness(snap, t0 + timedelta(hours=1), 86400), "fresh")
        self.assertEqual(fs.staleness(snap, t0 + timedelta(days=2), 86400), "stale")
        self.assertEqual(fs.staleness(snap, t0 - timedelta(hours=1), 86400), "unknown")
        self.assertEqual(fs.staleness(snap, t0 - timedelta(seconds=60), 86400), "fresh")
        self.assertEqual(fs.staleness(snap, t0 - timedelta(seconds=300), 86400), "fresh")
        self.assertEqual(fs.staleness(snap, t0 - timedelta(seconds=301), 86400), "unknown")

    def test_staleness_rejects_bad_snapshot(self):
        with self.assertRaises(fs.SnapshotError):
            fs.staleness({"schemaVersion": 2}, datetime.now(timezone.utc), 1)


class CliSmoke(unittest.TestCase):
    def run_cli(self, *args):
        env = dict(os.environ, PYTHONDONTWRITEBYTECODE="1")
        return subprocess.run([sys.executable, "-I", "-B", os.path.join(ROOT, "fleet_snapshot.py"), *args],
                              capture_output=True, text=True, env=env, cwd=ROOT)

    def test_cli_writes_golden_atomically(self):
        with tempfile.TemporaryDirectory(dir=ROOT) as td:
            out = os.path.join(td, "snap.json")
            # The host denylist is tested above with explicit values; here it
            # would make the golden depend on the test machine's name.
            r = self.run_cli(FIXTURE, "--out", out, "--no-host-denylist")
            self.assertEqual(r.returncode, 0, r.stderr)
            with open(out) as a, open(GOLDEN) as b:
                self.assertEqual(a.read(), b.read())
            self.assertEqual(os.listdir(td), ["snap.json"])

    def test_cli_refuses_without_opt_in_and_writes_nothing(self):
        with tempfile.TemporaryDirectory(dir=ROOT) as td:
            doc = load(); doc["exportEnabled"] = False
            inp = os.path.join(td, "in.json")
            with open(inp, "w") as f:
                json.dump(doc, f)
            out = os.path.join(td, "snap.json")
            r = self.run_cli(inp, "--out", out)
            self.assertEqual(r.returncode, 3)
            self.assertFalse(os.path.exists(out))

    def test_cli_malformed_input_exits_2_without_traceback(self):
        doc = json.dumps(load())
        cases = (b"\xff\xfe", b"[" * 100000, b'{"x": NaN}',
                 doc.replace('"exportEnabled": true', '"exportEnabled": false, "exportEnabled": true',
                             1).encode())
        self.assertIn(b"false, ", cases[-1])
        with tempfile.TemporaryDirectory(dir=ROOT) as td:
            inp = os.path.join(td, "in.json")
            for raw in cases:
                with open(inp, "wb") as f:
                    f.write(raw)
                r = self.run_cli(inp)
                with self.subTest(raw=raw[:20]):
                    self.assertEqual(r.returncode, 2, r.stderr)
                    self.assertNotIn("Traceback", r.stderr)
                    self.assertEqual(r.stdout, "")

    def test_cli_unwritable_output_exits_4_and_leaves_nothing(self):
        with tempfile.TemporaryDirectory(dir=ROOT) as td:
            out = os.path.join(td, "missing-dir", "snap.json")
            r = self.run_cli(FIXTURE, "--out", out, "--no-host-denylist")
            self.assertEqual(r.returncode, 4, r.stderr)
            self.assertNotIn("Traceback", r.stderr)
            self.assertNotIn(td, r.stderr)
            self.assertEqual(os.listdir(td), [])
            target = os.path.join(td, "snap.json")
            os.mkdir(target)  # os.replace onto a directory fails after the temp write
            r = self.run_cli(FIXTURE, "--out", target, "--no-host-denylist")
            self.assertEqual(r.returncode, 4, r.stderr)
            self.assertEqual(os.listdir(td), ["snap.json"])
            self.assertEqual(os.listdir(target), [])

    def test_cli_error_does_not_echo_paths(self):
        with tempfile.TemporaryDirectory(dir=ROOT) as td:
            doc = load(); doc["projects"] = {"/synthetic/home/u1/work/project-one": "bad/label"}
            inp = os.path.join(td, "in.json")
            with open(inp, "w") as f:
                json.dump(doc, f)
            r = self.run_cli(inp)
            self.assertEqual(r.returncode, 2)
            self.assertEqual(r.stdout, "")
            self.assertNotIn("/synthetic", r.stderr)


if __name__ == "__main__":
    unittest.main()
