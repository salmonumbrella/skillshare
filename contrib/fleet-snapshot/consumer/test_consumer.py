"""Synthetic contract and hostile-input tests; all temporary writes stay here."""
import contextlib
import copy
from datetime import datetime, timezone
import io
import json
from pathlib import Path
import tempfile
import unittest

import consumer as c

ROOT = Path(__file__).resolve().parent
FIXTURES = ROOT / "fixtures"
NOW = datetime(2026, 11, 16, 12, tzinfo=timezone.utc)


class ConsumerTests(unittest.TestCase):
    def setUp(self):
        self.registry = c.validate_policy(c.read_json(FIXTURES / "labels.json"))
        self.fresh = c.read_json(FIXTURES / "fresh.json")
        self.stale = c.read_json(FIXTURES / "stale.json")

    def report(self, snapshots, **kwargs):
        return c.build_report(snapshots, self.registry, NOW, **kwargs)

    def test_fresh_stale_missing_and_recorded_states_are_distinct(self):
        machines = self.report([self.stale, self.fresh])["machines"]
        self.assertEqual([m["snapshotStatus"] for m in machines], ["fresh", "stale", "missing"])
        self.assertEqual([e["state"] for e in machines[0]["entries"]], ["configured", "applied"])
        self.assertEqual(machines[1]["entries"][0]["state"], "unknown")
        self.assertEqual(machines[2]["entries"], [])

    def test_independent_exporter_golden_interoperates_without_rewriting(self):
        registry = c.validate_policy(c.read_json(FIXTURES / "exporter-labels.json"))
        snapshot = c.read_json(FIXTURES / "exporter-golden.json")
        report = c.build_report([snapshot], registry, NOW)
        fresh, missing = report["machines"]
        self.assertEqual(fresh["snapshotStatus"], "fresh")
        self.assertEqual(missing["snapshotStatus"], "missing")
        self.assertEqual(len(fresh["entries"]), 10)
        self.assertEqual({state: sum(e["state"] == state for e in fresh["entries"])
                          for state in c.STATES},
                         {"applied": 5, "configured": 3, "unknown": 2})
        self.assertIn({"skillId": "skill-a", "harnesses": ["agents", "claude"],
                       "state": "applied"}, fresh["entries"])

    def test_stale_applied_stays_recorded_applied(self):
        self.stale["entries"][0]["state"] = "applied"
        report = self.report([self.stale])["machines"][1]
        self.assertEqual(report["snapshotStatus"], "stale")
        self.assertEqual(report["entries"][0]["state"], "applied")

    def test_no_snapshot_means_missing_not_configured_or_unknown_entry(self):
        self.assertTrue(all(m["snapshotStatus"] == "missing" for m in self.report([])["machines"]))

    def test_boundary_and_fractional_utc(self):
        self.fresh["generatedAt"] = "2026-11-15T12:00:00.000000Z"
        self.assertEqual(self.report([self.fresh])["machines"][0]["snapshotStatus"], "fresh")
        self.assertEqual(self.report([self.fresh], max_age_seconds=86_399)["machines"][0]["snapshotStatus"], "stale")

    def test_duplicate_machine_even_identical_or_newer_rejected(self):
        for generated in (self.fresh["generatedAt"], "2026-11-16T11:00:00Z"):
            duplicate = copy.deepcopy(self.fresh)
            duplicate["generatedAt"] = generated
            with self.subTest(generated=generated), self.assertRaises(c.UnsafeInput):
                self.report([self.fresh, duplicate])

    def test_invalid_metadata_rejects_whole_batch(self):
        cases = [None, [], "snapshot", {}, {**self.fresh, "schemaVersion": True},
                 {**self.fresh, "schemaVersion": 2}, {**self.fresh, "schemaVersion": 1.0},
                 {**self.fresh, "entries": {}}, {**self.fresh, "hostPath": "synthetic"},
                 {**self.fresh, "machineId": "unregistered"}]
        for value in cases:
            with self.subTest(value=value), self.assertRaises(c.UnsafeInput):
                self.report([self.stale, value])

    def test_future_within_skew_is_fresh_beyond_is_unknown_with_recorded_states(self):
        for generated, status in (("2026-11-16T12:05:00Z", "fresh"),
                                  ("2026-11-16T12:05:01Z", "unknown"),
                                  ("2026-11-17T10:00:00Z", "unknown")):
            snapshot = {**self.fresh, "generatedAt": generated}
            with self.subTest(generated=generated):
                machine = self.report([snapshot, self.stale])["machines"][0]
                self.assertEqual(machine["snapshotStatus"], status)
                expected = sorted(self.fresh["entries"], key=lambda e: e["skillId"])
                self.assertEqual(machine["entries"], expected)

    def test_timestamps_must_be_real_utc(self):
        for value in (None, "2026-11-16", "2026-11-16T10:00:00", "2026-11-16T10:00:00+00:00",
                      "2026-02-30T10:00:00Z", "2026-11-16T10:00:60Z",
                      "2026-11-16T10:00:00.1234567Z", "2026-11-16T10:00:00Z\n"):
            with self.subTest(value=value), self.assertRaises(c.UnsafeInput):
                self.report([{**self.fresh, "generatedAt": value}])

    def test_payload_paths_urls_emails_and_content_not_accepted_in_any_label(self):
        # Entirely invented strings; no user paths or real identities.
        unsafe = ("/synthetic/target", "../synthetic", "~/synthetic", "C:\\synthetic",
                  "https://example.invalid/skill", "user@example.invalid", "host.local",
                  "token secret", "name\nsecond", "猫", "", "a" * 65)
        for field in ("machineId", "skillId", "projectLabel", "harnesses"):
            for value in unsafe:
                candidate = copy.deepcopy(self.fresh)
                if field == "machineId":
                    candidate[field] = value
                else:
                    candidate["entries"][0][field] = [value] if field == "harnesses" else value
                with self.subTest(field=field, value=value), self.assertRaises(c.UnsafeInput):
                    self.report([candidate])

    def test_unsafe_registry_cannot_bless_paths(self):
        for field in ("machineIds", "skillIds", "projectLabels", "harnesses"):
            registry = {key: sorted(value) for key, value in self.registry.items()}
            registry[field].append("/synthetic/target")
            with self.subTest(field=field), self.assertRaises(c.UnsafeInput):
                c.validate_policy(registry)

    def test_invalid_entries(self):
        original = self.fresh["entries"][0]
        cases = [None, {}, {**original, "state": "installed"}, {**original, "state": True},
                 {**original, "harnesses": []}, {**original, "harnesses": "harness-a"},
                 {**original, "harnesses": ["harness-a", "harness-a"]},
                 {**original, "harnesses": ["harness-b", "harness-a"]},
                 {**original, "projectLabel": None}, {**original, "skillId": "unregistered"},
                 {**original, "manifest": {}}, {**original, "targetDir": "/synthetic"}]
        for entry in cases:
            with self.subTest(entry=entry), self.assertRaises(c.UnsafeInput):
                self.report([{**self.fresh, "entries": [entry]}])

    def test_duplicate_entry_ignores_state(self):
        entry = copy.deepcopy(self.fresh["entries"][0])
        entry["state"] = "applied"
        with self.assertRaises(c.UnsafeInput):
            self.report([{**self.fresh, "entries": [self.fresh["entries"][0], entry]}])

    def test_same_skill_distinct_project_or_harness_group_is_preserved(self):
        entry = copy.deepcopy(self.fresh["entries"][0])
        entry.pop("projectLabel")
        self.fresh["entries"].append(entry)
        self.assertEqual(len(self.report([self.fresh])["machines"][0]["entries"]), 3)

    def test_invalid_report_controls(self):
        for age in (0, -1, float("nan"), float("inf"), True, "10"):
            with self.subTest(age=age), self.assertRaises(c.UnsafeInput):
                self.report([self.fresh], max_age_seconds=age)
        with self.assertRaises(c.UnsafeInput):
            c.build_report([self.fresh], self.registry, datetime(2026, 11, 16))

    def test_unsafe_json_duplicate_keys_constants_and_invalid_encoding(self):
        with tempfile.TemporaryDirectory(dir=ROOT) as directory:
            path = Path(directory) / "input.json"
            for raw in (b'{"schemaVersion":1,"schemaVersion":1}', b'{"x":NaN}', b'{"x":Infinity}',
                        b'{', b'\xff', b'[' * 2000, b'9' * 5000, b' ' * (c.MAX_BYTES + 1)):
                path.write_bytes(raw)
                with self.subTest(raw=raw[:40]), self.assertRaises(c.UnsafeInput):
                    c.read_json(path)

    def test_registry_shape_and_duplicates_fail_closed(self):
        base = {key: sorted(value) for key, value in self.registry.items()}
        cases = [None, {}, {**base, "machineIds": []}, {**base, "extra": []},
                 {**base, "skillIds": "skill-alpha"}, {**base, "machineIds": [True]},
                 {**base, "harnesses": ["harness-a", "harness-a"]}]
        for value in cases:
            with self.subTest(value=value), self.assertRaises(c.UnsafeInput):
                c.validate_policy(value)

    def test_no_symlink_directory_fifo_or_missing_file_reads(self):
        with tempfile.TemporaryDirectory(dir=ROOT) as directory:
            root = Path(directory)
            (root / "link").symlink_to(FIXTURES / "fresh.json")
            c.os.mkfifo(root / "fifo")
            for path in (root, root / "link", root / "fifo", root / "missing"):
                with self.subTest(path=path.name), self.assertRaises(c.UnsafeInput):
                    c.read_json(path)

    def test_cli_success_and_fail_closed_with_sanitized_error(self):
        args = ["--labels", str(FIXTURES / "labels.json"), "--now", "2026-11-16T12:00:00Z"]
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            code = c.main(args + [str(FIXTURES / "fresh.json"), str(FIXTURES / "stale.json")])
        self.assertEqual(code, 0)
        self.assertEqual(stderr.getvalue(), "")
        self.assertEqual(len(json.loads(stdout.getvalue())["machines"]), 3)
        with tempfile.TemporaryDirectory(dir=ROOT) as directory:
            path = Path(directory) / "synthetic.json"
            path.write_text(json.dumps({**self.fresh, "machineId": "/synthetic/secret"}))
            stdout, stderr = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                code = c.main(args + [str(FIXTURES / "fresh.json"), str(path)])
            self.assertEqual(code, 2)
            self.assertEqual(stdout.getvalue(), "")
            self.assertNotIn("/synthetic/secret", stderr.getvalue())
            self.assertNotIn(str(path), stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
