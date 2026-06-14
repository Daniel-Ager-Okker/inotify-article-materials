#!/usr/bin/env python3
"""Create a flat directory with many regular files (each with payload)."""

from __future__ import annotations

import argparse
import os
import secrets
import sys

DEFAULT_FILES = 200_000
DEFAULT_OUT = "fixture_200k"


def file_payload(index: int, seed: int) -> bytes:
    return (
        f"index={index}\n"
        f"seed={seed}\n"
        f"path_component=file_{index:06d}.dat\n"
    ).encode()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("-out", default=DEFAULT_OUT, help="directory to create")
    parser.add_argument("-n", type=int, default=DEFAULT_FILES, help="number of files to create")
    parser.add_argument("-seed", type=int, default=None, help="RNG seed for reproducible names (default: random)")
    args = parser.parse_args()

    if args.n < 1:
        print("-n must be >= 1", file=sys.stderr)
        return 1

    seed = args.seed if args.seed is not None else secrets.randbits(63)
    out = os.path.abspath(args.out)

    if os.path.exists(out):
        import shutil

        shutil.rmtree(out)
    os.makedirs(out, mode=0o755)

    for i in range(args.n):
        path = os.path.join(out, f"file_{i:06d}.dat")
        with open(path, "xb") as f:
            f.write(file_payload(i, seed))

    print(f"dir: {out}")
    print(f"seed: {seed}")
    print(f"files: {args.n}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
