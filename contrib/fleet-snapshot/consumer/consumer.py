#!/usr/bin/env python3
"""Read explicitly supplied, redacted v1 snapshots; never inspect skill targets."""
import argparse
from datetime import datetime, timezone
import json
import math
import os
import re
import stat
import sys

MAX_BYTES = 1_048_576
MAX_SKEW_SECONDS = 300  # shared with the exporter: further ahead is unknown
LABEL = re.compile(r"[A-Za-z][A-Za-z0-9_-]{0,63}\Z")
UTC = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?Z\Z")
STATES = {"configured", "applied", "unknown"}


class UnsafeInput(ValueError):
    """Input rejected; messages deliberately exclude untrusted values and paths."""


def require(condition, message):
    if not condition:
        raise UnsafeInput(message)


def keys(value, required, optional=()):
    require(type(value) is dict, "object required")
    require(set(required) <= value.keys() <= set(required) | set(optional),
            "missing or unsupported fields")


def label(value):
    require(type(value) is str and LABEL.fullmatch(value) is not None,
            "unsafe label")
    return value


def timestamp(value):
    require(type(value) is str and UTC.fullmatch(value) is not None,
            "UTC timestamp required")
    try:
        return datetime.fromisoformat(value[:-1] + "+00:00")
    except ValueError:
        raise UnsafeInput("invalid timestamp") from None


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate JSON key")
        result[key] = value
    return result


def invalid_constant(_):
    raise UnsafeInput("nonstandard JSON constant")


def read_json(path):
    """Only explicit regular files. Do not follow final symlinks or wait on FIFOs."""
    descriptor = None
    try:
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        metadata = os.fstat(descriptor)
        require(stat.S_ISREG(metadata.st_mode), "regular file required")
        require(metadata.st_size <= MAX_BYTES, "input too large")
        with os.fdopen(descriptor, "rb") as stream:
            descriptor = None
            raw = stream.read(MAX_BYTES + 1)
        require(len(raw) <= MAX_BYTES, "input too large")
        return json.loads(raw.decode("utf-8"), object_pairs_hook=unique_object,
                          parse_constant=invalid_constant)
    except UnsafeInput:
        raise
    except (OSError, UnicodeError, ValueError, RecursionError):
        raise UnsafeInput("cannot read safe JSON input") from None
    finally:
        if descriptor is not None:
            os.close(descriptor)


def validate_policy(value):
    """Registry is supplied by the caller; no discovery or user configuration."""
    keys(value, ("machineIds", "skillIds", "projectLabels", "harnesses"))
    result = {}
    for field, values in value.items():
        require(type(values) is list and len(values) <= 10_000, "invalid registry")
        for item in values:
            label(item)
        require(len(values) == len(set(values)), "duplicate registry label")
        result[field] = set(values)
    require(bool(result["machineIds"]), "machine registry is empty")
    return result


def allowed(value, registry, field):
    label(value)
    require(value in registry[field], "label absent from explicit registry")


def validate_snapshot(value, registry):
    keys(value, ("schemaVersion", "machineId", "generatedAt", "entries"))
    require(type(value["schemaVersion"]) is int and value["schemaVersion"] == 1,
            "unsupported schema version")
    allowed(value["machineId"], registry, "machineIds")
    timestamp(value["generatedAt"])
    entries = value["entries"]
    require(type(entries) is list and len(entries) <= 10_000, "invalid entries")
    seen = set()
    for entry in entries:
        keys(entry, ("skillId", "harnesses", "state"), ("projectLabel",))
        allowed(entry["skillId"], registry, "skillIds")
        if "projectLabel" in entry:
            allowed(entry["projectLabel"], registry, "projectLabels")
        require(type(entry["state"]) is str and entry["state"] in STATES,
                "invalid state")
        harnesses = entry["harnesses"]
        require(type(harnesses) is list and 1 <= len(harnesses) <= 32,
                "invalid harnesses")
        for harness in harnesses:
            allowed(harness, registry, "harnesses")
        require(len(harnesses) == len(set(harnesses)), "duplicate harness")
        require(harnesses == sorted(harnesses), "harnesses must be sorted")
        identity = (entry["skillId"], entry.get("projectLabel"), tuple(sorted(harnesses)))
        require(identity not in seen, "duplicate entry")
        seen.add(identity)
    return value


def build_report(snapshots, registry, now, max_age_seconds=86_400):
    require(isinstance(now, datetime) and now.tzinfo is not None
            and now.utcoffset().total_seconds() == 0, "UTC report time required")
    require(type(max_age_seconds) in (int, float) and math.isfinite(max_age_seconds)
            and max_age_seconds > 0, "positive finite max age required")
    require(type(snapshots) is list and len(snapshots) <= 256, "too many snapshots")
    machines = {}
    # Validate the entire batch before constructing any output.
    for candidate in snapshots:
        snapshot = validate_snapshot(candidate, registry)
        machine = snapshot["machineId"]
        require(machine not in machines, "duplicate machine snapshots")
        age = (now - timestamp(snapshot["generatedAt"])).total_seconds()
        machines[machine] = (snapshot, age)
    output = []
    for machine in sorted(registry["machineIds"]):
        if machine not in machines:
            output.append({"machineId": machine, "snapshotStatus": "missing", "entries": []})
            continue
        snapshot, age = machines[machine]
        if age < -MAX_SKEW_SECONDS:
            status = "unknown"  # clock skew: freshness cannot be judged
        else:
            status = "stale" if age > max_age_seconds else "fresh"
        output.append({"machineId": machine, "snapshotStatus": status,
                       "generatedAt": snapshot["generatedAt"], "ageSeconds": age,
                       "entries": sorted(snapshot["entries"], key=lambda e: (
                           e["skillId"], e.get("projectLabel", ""), tuple(sorted(e["harnesses"]))))})
    return {"reportVersion": 1, "machines": output}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--labels", required=True, help="explicit alias registry JSON file")
    parser.add_argument("--now", help="UTC report time, default current UTC")
    parser.add_argument("--max-age-seconds", type=float, default=86_400)
    parser.add_argument("snapshots", nargs="*", help="explicit snapshot JSON files")
    args = parser.parse_args(argv)
    try:
        registry = validate_policy(read_json(args.labels))
        now = timestamp(args.now) if args.now is not None else datetime.now(timezone.utc)
        require(len(args.snapshots) <= 256, "too many snapshots")
        result = build_report([read_json(path) for path in args.snapshots], registry,
                              now, args.max_age_seconds)
    except UnsafeInput as error:
        print("snapshot report rejected: " + str(error), file=sys.stderr)
        return 2
    print(json.dumps(result, indent=2, sort_keys=True, allow_nan=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
