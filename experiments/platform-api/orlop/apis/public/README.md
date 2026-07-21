# Auto-Generated Public API

**WARNING: This directory is auto-generated. Do not modify files manually.**

This directory contains the public API types generated from the internal API definitions.
All files in this directory are created by the orlop-gen code generator based on the
source files in the internal API directory.

## Regenerating

To regenerate this directory, run:

```bash
make generate
# or
go run ./cmd/orlop-gen
```

Any manual changes will be lost when the generator is run again.

## Source

The source files are located in the internal API directory. To make changes:

1. Modify the internal API types
2. Add or update +orlop:public markers on fields that should be public
3. Run the code generator to update this directory

## Generated Files

This directory contains:
- Filtered API types based on +orlop:public markers
- Generated DeepCopy methods (zz_generated.deepcopy.go)
- Generated OpenAPI v3 schemas (zz_generated.schemas.go)
- Generated conversion functions (zz_generated.conversion.go)
- API version aggregator files with updated import paths
