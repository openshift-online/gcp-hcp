#!/usr/bin/env python3
"""Validate the Config Sync `config.kubernetes.io/depends-on` graph of a bundle.

Input: a JSON array (or stream) of the final, kustomized objects. Fails closed
(exit 1, reason on stderr) unless:

  * at least one dependency edge is declared (zero edges is a failure);
  * every token uses the cli-utils syntax (`<group>/<Kind>/<name>` for
    cluster-scoped objects, `<group>/namespaces/<ns>/<Kind>/<name>` otherwise)
    and resolves to an object declared in the same input;
  * no dependency target carries `client.lifecycle.config.k8s.io/mutation:
    ignore`: in the Config Sync v1.23.3 applier (cli-utils 5895ad6c17dd) an
    existing, mutation-ignored object is a skipped apply, and the dependency
    filter then skips every dependent on each later sync;
  * the whole directed graph is acyclic (any cycle, of any length).

Usage: python3 -I check-depends-on.py <objects.json>
"""
import json
import sys

DEPENDS_ON = "config.kubernetes.io/depends-on"
MUTATION = "client.lifecycle.config.k8s.io/mutation"


def load(path):
    text = open(path, encoding="utf-8").read()
    dec, i, out = json.JSONDecoder(), 0, []
    while i < len(text):
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            break
        obj, i = dec.raw_decode(text, i)
        out.extend(obj if isinstance(obj, list) else [obj])
    return [o for o in out if o]


def ident(o):
    api = o["apiVersion"]
    group = api.split("/")[0] if "/" in api else ""
    md = o["metadata"]
    return (group, o["kind"], md.get("namespace") or "", md["name"])


def parse(token):
    f = token.strip().split("/")
    if len(f) == 3:
        return (f[0], f[1], "", f[2])
    if len(f) == 5 and f[1] == "namespaces":
        return (f[0], f[3], f[2], f[4])
    raise ValueError("malformed depends-on token %r" % token)


def main(path):
    objs = load(path)
    by_id = {ident(o): o for o in objs}
    edges = {}
    count = 0
    errs = []
    for o in objs:
        raw = (o["metadata"].get("annotations") or {}).get(DEPENDS_ON)
        if raw is None:
            continue
        for tok in raw.split(","):
            try:
                tgt = parse(tok)
            except ValueError as e:
                errs.append(str(e))
                continue
            count += 1
            if tgt not in by_id:
                errs.append("%s depends on %s which is not declared in the bundle" % (ident(o), tok))
                continue
            if (by_id[tgt]["metadata"].get("annotations") or {}).get(MUTATION) == "ignore":
                errs.append("%s depends on %s, which is mutation:ignore (dependents would be skipped on every later sync)" % (ident(o)[1:], tok))
            edges.setdefault(ident(o), set()).add(tgt)
    if count == 0:
        errs.append("no depends-on edge is declared")
    # Kahn topological sort: leftover nodes are on or behind a cycle.
    nodes = set(edges) | {t for ts in edges.values() for t in ts}
    indeg = {n: 0 for n in nodes}
    for n, ts in edges.items():
        for t in ts:
            indeg[n] += 1  # n waits for t
    ready = [n for n, d in indeg.items() if d == 0]
    dependents = {}
    for n, ts in edges.items():
        for t in ts:
            dependents.setdefault(t, set()).add(n)
    seen = 0
    while ready:
        n = ready.pop()
        seen += 1
        for d in dependents.get(n, ()):
            indeg[d] -= 1
            if indeg[d] == 0:
                ready.append(d)
    if seen != len(nodes):
        stuck = sorted("%s/%s" % (n[1], n[3]) for n, d in indeg.items() if d > 0)
        errs.append("dependency cycle involving: " + ", ".join(stuck))
    if errs:
        sys.exit("check-depends-on.py: " + "; ".join(errs))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    main(sys.argv[1])
