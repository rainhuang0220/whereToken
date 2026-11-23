#!/usr/bin/env python3
"""Summarize the benchmark and measure the scar against its flat photo."""
import argparse
import json
import math
from pathlib import Path
from statistics import mean, median


def distribution(values):
    values = sorted(values)
    if not values:
        return {"count": 0}
    return {
        "count": len(values),
        "median": round(median(values), 2),
        "p95": round(values[math.ceil(.95 * len(values)) - 1], 2),
        "max": round(values[-1], 2),
    }


def matching_frame(frame):
    if not frame["physical"]:
        return True
    width, height = frame["domWidth"], frame["domHeight"]
    return (
        frame["revision"] == frame["currentRevision"]
        and height == frame["committedHeight"] == frame["meshHeight"]
        and width == frame["meshWidth"]
        and frame["textureSize"] == [int(width * frame["scale"]), int(height * frame["scale"])]
    )


def performance(directory):
    measurements = json.loads((directory / "measurements.json").read_text())
    cold = measurements["cold"]
    last = cold[-1]["timeline"]
    operations = measurements["operations"]
    interactions = [click for click in last["clicks"] if "frame" in click]
    first_click = min(click["start"] for click in interactions)
    tasks = [entry["duration"] for entry in last["tasks"] if entry["start"] >= first_click]
    frames = [frame for run in cold for frame in run["timeline"]["frames"]]
    timings = ["captureMs", "materialMs", "geometryMs", "compileMs", "uploadMs", "drawMs", "bitmapMs"]
    return {
        "cold": [
            {
                "nativeMs": run["nativeMs"],
                "committedMs": run["readyMs"],
                "firstNativeDataFrameMs": next((f["visibleSince"] for f in run["timeline"]["frames"]
                                                if f["visibleSince"] >= 0), None),
                "longTasks": [event["duration"] for event in run["timeline"]["tasks"]
                              if not run["timeline"]["clicks"] or
                              event["start"] < run["timeline"]["clicks"][0]["start"]],
                "phases": {key: run["diagnostic"].get(key) for key in timings},
            }
            for run in cold
        ],
        "nativeClickDispatchMs": distribution([c["dispatch"] - c["start"] for c in interactions]),
        "nativeClickToAnimationFrameMs": distribution([c["frame"] - c["start"] for c in interactions]),
        # Chromium reports entries only at or above the configured 16 ms threshold.
        "reportedClickEventDurationMs": distribution(
            [e["duration"] for e in last["events"] if e["name"] == "click"]
        ),
        "automationDispatchWallMs": distribution([op["dispatchWallMs"] for op in operations]),
        "physicalCommitWallMs": distribution([op["readyMs"] for op in operations]),
        "workerAndCaptureMs": {
            key: distribution([op["diagnostic"][key] for op in operations if key in op["diagnostic"]])
            for key in timings
        },
        "steadyLongTasks": tasks,
        "samplesByRange": {
            label: distribution([op["readyMs"] for op in operations if op["label"] == label])
            for label in dict.fromkeys(op["label"] for op in operations)
        },
        "rapidClickWallMs": measurements["rapid"]["nativeMs"],
        "rapidCommitWallMs": measurements["rapid"]["readyMs"],
        "sampledFrames": len(frames),
        "inconsistentFrames": sum(not matching_frame(frame) for frame in frames),
    }


def scar_measurements(directory):
    from PIL import Image
    manifest = json.loads((directory / "capture-manifest.json").read_text())
    results = {}
    for name, state in manifest.items():
        flat = Image.open(directory / f"{name}-flat.png").convert("L")
        released = Image.open(directory / f"{name}-released.png").convert("L")
        memory = state["memories"]
        hinge = memory["memories"][0]["hinge"]

        def coordinates(point):
            return point["x"] * 100 + flat.width / 2, memory["height"] * 50 - point["y"] * 100

        def difference(x, y):
            if 0 <= x < flat.width and 0 <= y < flat.height:
                return flat.getpixel((x, y)) - released.getpixel((x, y))
            return None

        start, end = coordinates(hinge["start"]), coordinates(hinge["end"])
        darkness, outside = [], []
        for percent in range(10, 91):
            x = start[0] + (end[0] - start[0]) * percent / 100
            y = start[1] + (end[1] - start[1]) * percent / 100
            differences = [difference(round(x + offset * hinge["nx"]), round(y - offset * hinge["ny"]))
                           for offset in range(-8, 9)]
            darkness.append(max(value for value in differences if value is not None))
            for offset in (-30, 30):
                value = difference(round(x + offset * hinge["nx"]), round(y - offset * hinge["ny"]))
                if value is not None:
                    outside.append(abs(value))
        corner = []
        for y in range(max(0, int(min(start[1], end[1]) - 30)),
                       min(flat.height, int(max(start[1], end[1]) + 30))):
            for x in range(max(0, int(min(start[0], end[0]) - 30)),
                           min(flat.width, int(max(start[0], end[0]) + 30))):
                offset = (x - start[0]) * hinge["nx"] - (y - start[1]) * hinge["ny"]
                if abs(offset) > 22:
                    corner.append(abs(difference(x, y)))
        results[name] = {
            "meanCreaseDarkness": round(mean(darkness), 2),
            "maxOutsideCreaseDelta": max(outside),
            "cornerPixelsOutsideCrease": len(corner),
            "cornerPixelsChangedByMoreThan3": sum(delta > 3 for delta in corner),
        }
    return results


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", type=Path, help="directory with benchmark measurements")
    parser.add_argument("--scars-before", type=Path)
    parser.add_argument("--scars-after", type=Path)
    args = parser.parse_args()
    summary = performance(args.directory)
    if args.scars_before and args.scars_after:
        before, after = (scar_measurements(directory) for directory in (args.scars_before, args.scars_after))
        summary["scarContrast"] = {
            key: {
                "before": baseline,
                "after": after[key],
                "relativeDarkness": round(after[key]["meanCreaseDarkness"] / baseline["meanCreaseDarkness"], 3),
            }
            for key, baseline in before.items()
        }
    print(json.dumps(summary, indent=2))
    if summary["inconsistentFrames"]:
        raise SystemExit("a visible frame did not match its document, mesh, or print revision")
    if summary["reportedClickEventDurationMs"].get("p95", math.inf) >= 100:
        raise SystemExit("native click feedback exceeded the p95 budget")
    if any(value > 100 for value in summary["steadyLongTasks"]):
        raise SystemExit("a filter produced a main-thread task longer than 100 ms")


if __name__ == "__main__":
    main()
