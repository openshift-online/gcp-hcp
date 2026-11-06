#!/usr/bin/env python3
"""Minimal structural validation of custom resources against CRD schemas.

Rejects unknown fields (what structural pruning would silently drop), missing
required fields, wrong primitive types and enum violations. It is a typo and
drift guard for the generated bundle, not a full OpenAPI validator.

Usage: python3 -I validate_cr.py <crds.json> <crd-name> <version> <cr.json>
Inputs are JSON (use yq -o=json): a stream/array of CRDs, and a stream/array
of custom resources of that CRD's kind.
"""
import json
import sys

TYPES = {
    "string": str,
    "boolean": bool,
    "integer": int,
    "number": (int, float),
    "array": list,
    "object": dict,
}


def load_many(path):
    text = open(path, encoding="utf-8").read()
    dec = json.JSONDecoder()
    out, i = [], 0
    while i < len(text):
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            break
        obj, i = dec.raw_decode(text, i)
        out.extend(obj if isinstance(obj, list) else [obj])
    return out


def check(schema, value, path, errs):
    if schema.get("x-kubernetes-preserve-unknown-fields"):
        return
    if schema.get("x-kubernetes-int-or-string"):
        if not isinstance(value, (int, str)) or isinstance(value, bool):
            errs.append("%s: expected int-or-string" % path)
        return
    t = schema.get("type")
    if t:
        py = TYPES[t]
        ok = isinstance(value, py) and not (t in ("integer", "number") and isinstance(value, bool))
        if not ok:
            errs.append("%s: expected %s, got %s" % (path, t, type(value).__name__))
            return
    if "enum" in schema and value not in schema["enum"]:
        errs.append("%s: %r not in enum" % (path, value))
    if isinstance(value, dict):
        props = schema.get("properties")
        addl = schema.get("additionalProperties")
        for req in schema.get("required", []):
            if req not in value:
                errs.append("%s: missing required field %s" % (path, req))
        for k, v in value.items():
            if props and k in props:
                check(props[k], v, "%s.%s" % (path, k), errs)
            elif isinstance(addl, dict):
                check(addl, v, "%s.%s" % (path, k), errs)
            elif addl is not True:
                errs.append("%s: unknown field %s" % (path, k))
    elif isinstance(value, list) and "items" in schema:
        for i, v in enumerate(value):
            check(schema["items"], v, "%s[%d]" % (path, i), errs)


def main(crds_path, crd_name, version, cr_path):
    crds = [c for c in load_many(crds_path) if c and c.get("metadata", {}).get("name") == crd_name]
    if len(crds) != 1:
        sys.exit("validate_cr.py: CRD %s found %d times" % (crd_name, len(crds)))
    vers = [v for v in crds[0]["spec"]["versions"] if v["name"] == version]
    if len(vers) != 1:
        sys.exit("validate_cr.py: version %s not in CRD" % version)
    schema = vers[0]["schema"]["openAPIV3Schema"]
    errs = []
    for cr in load_many(cr_path):
        for k, v in cr.items():
            if k in ("apiVersion", "kind", "metadata"):
                continue
            if k not in schema.get("properties", {}):
                errs.append("%s: unknown top-level field" % k)
            else:
                check(schema["properties"][k], v, k, errs)
    if errs:
        sys.exit("validate_cr.py: " + "; ".join(errs[:10]))


if __name__ == "__main__":
    if len(sys.argv) != 5:
        sys.exit(__doc__)
    main(*sys.argv[1:])
