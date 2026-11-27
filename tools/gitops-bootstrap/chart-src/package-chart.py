#!/usr/bin/env python3
"""Deterministically package a Helm chart directory into <name>-<version>.tgz.

Fixed member order, mtime, ownership and modes, and a zero gzip header
timestamp, so the same sources always produce the same bytes (for a given
zlib). Usage: python3 -I package-chart.py <chart-dir> <output-dir>
"""
import gzip
import io
import os
import re
import sys
import tarfile


def main(chart_dir, out_dir):
    chart_dir = os.path.abspath(chart_dir)
    name = os.path.basename(chart_dir)
    version = None
    with open(os.path.join(chart_dir, "Chart.yaml"), encoding="utf-8") as f:
        for line in f:
            m = re.match(r"^version:\s*(\S+)\s*$", line)
            if m:
                version = m.group(1).strip("\"'")
    if not version:
        sys.exit("package-chart.py: no version in Chart.yaml")
    members = []
    for root, dirs, files in os.walk(chart_dir):
        dirs.sort()
        for fn in sorted(files):
            full = os.path.join(root, fn)
            rel = os.path.relpath(full, chart_dir)
            members.append((name + "/" + rel.replace(os.sep, "/"), full))
    members.sort()
    out_path = os.path.join(out_dir, "%s-%s.tgz" % (name, version))
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode="w", format=tarfile.USTAR_FORMAT) as tar:
        for arcname, full in members:
            info = tarfile.TarInfo(arcname)
            with open(full, "rb") as fh:
                data = fh.read()
            info.size = len(data)
            info.mtime = 0
            info.mode = 0o644
            info.uid = info.gid = 0
            info.uname = info.gname = ""
            tar.addfile(info, io.BytesIO(data))
    with open(out_path, "wb") as out:
        with gzip.GzipFile(filename="", mode="wb", fileobj=out, compresslevel=9, mtime=0) as gz:
            gz.write(raw.getvalue())
    print(out_path)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
