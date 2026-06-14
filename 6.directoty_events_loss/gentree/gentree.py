#!/usr/bin/env python3
"""Create a nested directory tree with many regular files (each with payload)."""

from __future__ import annotations

import argparse
import os
import random
import secrets
import shutil
import sys

DEFAULT_FILES = 500_000
DEFAULT_OUT = "fixture_1m"
DEFAULT_DEPTH = 6
DEFAULT_FANOUT = 12


def file_payload(index: int, seed: int, relpath: str) -> bytes:
    return (
        f"index={index}\n"
        f"seed={seed}\n"
        f"path={relpath}\n"
    ).encode()


def random_partition(rng: random.Random, total: int, parts: int) -> list[int]:
    """Split total into `parts` positive integers (sum == total)."""
    if parts <= 0:
        return []
    if parts == 1:
        return [total]
    if total < parts:
        return [1] * total + [0] * (parts - total)

    shares: list[int] = []
    remaining = total
    for i in range(parts - 1):
        slots_left = parts - i - 1
        max_share = remaining - slots_left
        share = rng.randint(1, max_share)
        shares.append(share)
        remaining -= share
    shares.append(remaining)
    return shares


class TreeBuilder:
    def __init__(
        self,
        root: str,
        file_count: int,
        seed: int,
        max_depth: int,
        fanout: int,
    ) -> None:
        self.root = root
        self.file_count = file_count
        self.seed = seed
        self.rng = random.Random(seed)
        self.max_depth = max_depth
        self.fanout = fanout
        self.file_index = 0
        self.dir_count = 0
        self.max_depth_reached = 0

    def build(self) -> None:
        os.makedirs(self.root, mode=0o755, exist_ok=True)
        self._grow(self.root, self.file_count, depth=0)

    def _write_files(self, directory: str, count: int) -> None:
        for _ in range(count):
            name = f"file_{self.file_index:06d}.dat"
            path = os.path.join(directory, name)
            relpath = os.path.relpath(path, self.root)
            with open(path, "xb") as handle:
                handle.write(file_payload(self.file_index, self.seed, relpath))
            self.file_index += 1

    def _grow(self, path: str, file_quota: int, depth: int) -> None:
        if file_quota <= 0:
            return

        self.max_depth_reached = max(self.max_depth_reached, depth)

        if depth >= self.max_depth:
            self._write_files(path, file_quota)
            return

        # Keep a few files at intermediate levels (real trees are not leaf-only).
        max_here = min(file_quota, self.rng.randint(0, max(1, file_quota // 50)))
        if max_here:
            self._write_files(path, max_here)
            file_quota -= max_here
            if file_quota <= 0:
                return

        if depth + 1 >= self.max_depth:
            self._write_files(path, file_quota)
            return

        subdir_count = min(
            self.fanout,
            file_quota,
            self.rng.randint(2, self.fanout),
        )
        quotas = random_partition(self.rng, file_quota, subdir_count)

        for idx, quota in enumerate(quotas):
            if quota <= 0:
                continue
            self.dir_count += 1
            subpath = os.path.join(path, f"dir_{depth + 1}_{idx:04d}")
            os.makedirs(subpath, mode=0o755, exist_ok=True)
            self._grow(subpath, quota, depth + 1)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("-out", default=DEFAULT_OUT, help="directory to create")
    parser.add_argument("-n", type=int, default=DEFAULT_FILES, help="number of files to create")
    parser.add_argument(
        "-depth",
        type=int,
        default=DEFAULT_DEPTH,
        help="maximum directory nesting depth (default: %(default)s)",
    )
    parser.add_argument(
        "-fanout",
        type=int,
        default=DEFAULT_FANOUT,
        help="target number of subdirectories per level (default: %(default)s)",
    )
    parser.add_argument("-seed", type=int, default=None, help="RNG seed for reproducible layout (default: random)")
    args = parser.parse_args()

    if args.n < 1:
        print("-n must be >= 1", file=sys.stderr)
        return 1
    if args.depth < 1:
        print("-depth must be >= 1", file=sys.stderr)
        return 1
    if args.fanout < 2:
        print("-fanout must be >= 2", file=sys.stderr)
        return 1

    seed = args.seed if args.seed is not None else secrets.randbits(63)
    out = os.path.abspath(args.out)

    if os.path.exists(out):
        shutil.rmtree(out)

    builder = TreeBuilder(out, args.n, seed, args.depth, args.fanout)
    builder.build()

    print(f"dir: {out}")
    print(f"seed: {seed}")
    print(f"files: {builder.file_index}")
    print(f"subdirs: {builder.dir_count}")
    print(f"max depth: {builder.max_depth_reached}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
