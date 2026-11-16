"""Exporter and consumer are written independently; these tests hold them to
one contract. Synthetic fixtures only; temporary files stay under this tree."""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
EXPORTER = os.path.join(ROOT, "exporter")
CONSUMER = os.path.join(ROOT, "consumer")
sys.path[:0] = [EXPORTER, CONSUMER]

import consumer as c  # noqa: E402
import fleet_snapshot as fs  # noqa: E402

INPUT = os.path.join(EXPORTER, "fixtures", "synthetic-machine.input.json")
GOLDEN = os.path.join(EXPORTER, "fixtures", "synthetic-machine.snapshot.golden.json")
LABELS = os.path.join(CONSUMER, "fixtures", "exporter-labels.json")
GENERATED = datetime(2026, 11, 16, 12, tzinfo=timezone.utc)


def entry_key(entry):
    return entry["skillId"], entry.get("projectLabel", ""), tuple(entry["harnesses"])


def run(script, *args):
    return subprocess.run([sys.executable, "-I", "-B", script, *args],
                          capture_output=True, text=True, cwd=ROOT)


def snapshot_with_skill(skill):
    return {"schemaVersion": 1, "machineId": "m-0a1b2c3d", "generatedAt": "2026-11-16T12:00:00Z",
            "entries": [{"skillId": skill, "harnesses": ["claude"], "state": "applied"}]}


def accepted_by_exporter(skill):
    try:
        fs.validate_snapshot(snapshot_with_skill(skill))
        return True
    except fs.SnapshotError:
        return False


def accepted_by_consumer(skill):
    try:
        registry = c.validate_policy({"machineIds": ["m-0a1b2c3d"], "skillIds": [skill],
                                      "projectLabels": [], "harnesses": ["claude"]})
        c.validate_snapshot(snapshot_with_skill(skill), registry)
        return True
    except c.UnsafeInput:
        return False


class EndToEnd(unittest.TestCase):
    def test_exporter_cli_output_is_reported_unchanged_by_consumer_cli(self):
        with tempfile.TemporaryDirectory(dir=ROOT) as tmp:
            snap_path = os.path.join(tmp, "snapshot.json")
            # The host denylist is covered by the exporter's own tests; it is
            # off here so the result does not depend on this machine's name.
            exported = run(os.path.join(EXPORTER, "fleet_snapshot.py"), INPUT,
                           "--out", snap_path, "--no-host-denylist")
            self.assertEqual(exported.returncode, 0, exported.stderr)
            with open(snap_path, encoding="utf-8") as f, open(GOLDEN, encoding="utf-8") as g:
                text = f.read()
                self.assertEqual(text, g.read())
            reported = run(os.path.join(CONSUMER, "consumer.py"), "--labels", LABELS,
                           "--now", "2026-11-16T13:00:00Z", snap_path)
            self.assertEqual(reported.returncode, 0, reported.stderr)
            self.assertEqual(reported.stderr, "")
        report = json.loads(reported.stdout)
        snapshot = json.loads(text)
        machines = {m["machineId"]: m for m in report["machines"]}
        self.assertEqual(set(machines), {"m-0a1b2c3d", "spoke-missing"})
        self.assertEqual(machines["spoke-missing"], {"machineId": "spoke-missing",
                                                     "snapshotStatus": "missing", "entries": []})
        machine = machines["m-0a1b2c3d"]
        self.assertEqual(machine["snapshotStatus"], "fresh")
        self.assertEqual(machine["ageSeconds"], 3600)
        # Recorded states pass through exactly; nothing is derived or dropped.
        self.assertEqual(sorted(machine["entries"], key=entry_key),
                         sorted(snapshot["entries"], key=entry_key))

    def test_consumer_fixture_is_the_exporter_golden(self):
        with open(GOLDEN, "rb") as a, open(os.path.join(CONSUMER, "fixtures",
                                                        "exporter-golden.json"), "rb") as b:
            self.assertEqual(a.read(), b.read())


class SharedContract(unittest.TestCase):
    def test_alias_grammar_is_identical(self):
        cases = ["skill-a", "Skill_A", "a", "a" + "b" * 38, "1skill", "skill.a", "skill a",
                 "skill/a", "~skill", "skill-a\n", "", "猫", "user@example.invalid",
                 "-skill", "_skill"]
        for skill in cases:
            with self.subTest(skill=skill):
                self.assertEqual(accepted_by_exporter(skill), accepted_by_consumer(skill))

    def test_token_shaped_aliases_are_a_writer_only_refusal(self):
        # The grammar allows 64 characters; the exporter alone also refuses
        # values long or prefixed enough to look like credentials.
        for skill in ("a" * 40, "a" * 64, "ghp_" + "x" * 10):
            with self.subTest(skill=skill):
                self.assertFalse(accepted_by_exporter(skill))
                self.assertTrue(accepted_by_consumer(skill))

    def test_future_skew_and_staleness_agree(self):
        with open(GOLDEN, encoding="utf-8") as f:
            snapshot = json.load(f)
        registry = c.validate_policy({"machineIds": ["m-0a1b2c3d"],
                                      "skillIds": ["skill-a", "skill-b", "skill-c", "skill-d",
                                                   "skill-orphan"],
                                      "projectLabels": ["alpha"],
                                      "harnesses": ["agents", "claude", "codex", "cursor",
                                                    "gemini"]})
        expected = {-301: "unknown", -300: "fresh", -1: "fresh", 0: "fresh",
                    86_400: "fresh", 86_401: "stale"}
        for offset, status in expected.items():
            now = GENERATED + timedelta(seconds=offset)
            with self.subTest(offset=offset):
                self.assertEqual(fs.staleness(snapshot, now, 86_400), status)
                report = c.build_report([snapshot], registry, now, 86_400)
                self.assertEqual(report["machines"][0]["snapshotStatus"], status)

    def test_unsorted_harnesses_are_refused_by_both(self):
        snapshot = snapshot_with_skill("skill-a")
        snapshot["entries"][0]["harnesses"] = ["codex", "claude"]
        with self.assertRaises(fs.SnapshotError):
            fs.validate_snapshot(snapshot)
        registry = c.validate_policy({"machineIds": ["m-0a1b2c3d"], "skillIds": ["skill-a"],
                                      "projectLabels": [], "harnesses": ["claude", "codex"]})
        with self.assertRaises(c.UnsafeInput):
            c.validate_snapshot(snapshot, registry)

    def test_duplicate_identity_is_refused_by_both(self):
        snapshot = snapshot_with_skill("skill-a")
        snapshot["entries"].append(dict(snapshot["entries"][0], state="configured"))
        with self.assertRaises(fs.SnapshotError):
            fs.validate_snapshot(snapshot)
        registry = c.validate_policy({"machineIds": ["m-0a1b2c3d"], "skillIds": ["skill-a"],
                                      "projectLabels": [], "harnesses": ["claude"]})
        with self.assertRaises(c.UnsafeInput):
            c.validate_snapshot(snapshot, registry)


if __name__ == "__main__":
    unittest.main()
