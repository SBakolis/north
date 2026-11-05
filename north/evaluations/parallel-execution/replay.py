"""Replay the retained native dispatch trace; not a scheduler or runtime benchmark."""

import json
from pathlib import Path


def validate_trace(data):
    active, launched, completed, reviewed = set(), set(), set(), set()
    peak = 0
    refill = False
    accepted = False
    for event in data["events"]:
        kind = event["event"]
        if kind in ("launch_batch", "launch"):
            tasks = event.get("tasks", [event.get("task")])
            assert all(task and task not in launched for task in tasks), "duplicate dispatch"
            assert not accepted, "launch after acceptance"
            if "C" in tasks and active:
                refill = True
            active.update(tasks)
            launched.update(tasks)
            peak = max(peak, len(active))
            assert len(active) <= data["max_parallel"], "concurrency budget exceeded"
        elif kind == "completion_notification":
            task = event["task"]
            assert task in active, "completion without active assignment"
            active.remove(task)
            completed.add(task)
        elif kind == "primary_review":
            assert event["task"] in completed, "review of unfinished artifact"
            reviewed.add(event["task"])
        elif kind == "accept_layer":
            assert not active and reviewed == launched == {"A", "B", "C"}
            accepted = True
    assert accepted, "layer acceptance missing"
    assert data["events"][0]["tasks"] == ["A", "B"], "initial sibling batch missing"
    return {
        "peak_outstanding_assignments": peak,
        "refill_before_other_completion_notification": refill,
    }


if __name__ == "__main__":
    root = Path(__file__).resolve().parent
    trace = json.loads((root / "events.json").read_text())
    result = validate_trace(trace)
    # The recorder is evidence supplied by the primary, not an independent host log.
    # Worker timestamps and reports are retained separately; do not infer speedups.
    print(json.dumps(result, indent=2))
