# Phase 4 live API guard rehearsal — blocked by patch-format incompatibility

**2026-11-19. Not passed. Do not run `iam update --confirm` against the
currently deployed public API.** Its assertion path uses JSONPatch arrays,
but that public handler currently accepts only merge-patch objects. The
wrong-UID request was rejected before the UID comparison was evaluated.
The stale-resource-version request was **not attempted**. No new assertion,
IAM change, gate activation, upgrade or server rollout occurred.

## Approved scope and source

The user approved: baseline reads, one wrong-UID negative PATCH, one stale-RV
negative PATCH, readback and evidence. The remaining replacement operation in
each request was the **identical existing** one-element application history;
no request could append an assertion or change a release. The dev environment
statement was not treated as authorization for a broader production fix.

- Bounded harness and six offline tests were reviewed by the architect and
  committed **before execution** as `3dfbb12`, then run unchanged:
  [`tools/api-guard-rehearsal.py`](tools/api-guard-rehearsal.py) and
  [`tools/test_api_guard_rehearsal.py`](tools/test_api_guard_rehearsal.py).
  All six offline tests passed, covering intent-before-PATCH, unchanged list,
  correct content type/host, no proxy/redirect/retry, authentication errors,
  failure-marker specificity and refusal to repeat an uncertain case.
- The harness uses authenticated public HTTPS with normal certificate
  validation, bearer tokens held only in memory, proxies/redirects disabled,
  and no automatic retry. It checks both gates false and private receipt/
  upgrade identity before every negative request. Error bodies/headers/tokens
  are not logged or stored; a digest and fixed failure markers are retained.
- Protected journal (0700 directory/0600 file):
  `/Users/asegundo/.local/state/gcp-hcp/wif-phase4-api-guards-20261119/guard-state.json`.
  Its state is **wrong-uid=UnexpectedResponse**, `rejectedByTest=false`; no
  `stale-rv` entry. SHA-256
  `af1f4abf8e822266f8a089bf03773afb8eb908083ca31f28f19e1a025d178588`.
  It remains unchanged. Do not clear/edit it, rerun the PATCH or reuse the
  single-shot harness. It is retained for provenance, not a runnable success
  recipe for this endpoint. A future attempt needs reviewed scope/new state.

## Actual results of the five steps

1. **Baseline passed at 21:17:04Z.** Authenticated public GET and independent
   private/gate reads identified original Cluster UID
   `6b152684-797c-4f17-a258-43538f8013c8`, generation 2, RV `401`,
   release 5.0.2/candidate, and the same single `5.0.0` template pin.
   Canonical public-spec digest:
   `sha256:d8d739a85da263b872e1d20c8189aa7a0a1440e213adebb63a9f9f7468669a26`.
   Template verified=false, application writes=false; original Dispatched
   receipt, no Pending receipt or requested upgrade. Guest was healthy.
2. **Wrong-UID case blocked at 21:17:14Z.** One HTTP PATCH used
   `application/json-patch+json`, first `test /metadata/uid` against the
   intentionally wrong all-zero UID, then `replace
   /spec/platform/gcp/iam/templateApplications` with the identical existing
   list. It returned **HTTP 400**, without the expected specific test-failure
   marker. Response-body SHA-256:
   `sha256:87f8744aed85e4dbe6c278298c284f85ce91695adaacd3228684e70c7e8c6c22`.
   The harness correctly refused to count an arbitrary 4xx as guard success
   and stopped. No retry or live diagnostic PATCH was sent.
3. **Stale-RV case not run**, because the first required guard was not proven.
4. **Independent read-only safety readback passed.** Immediately after the
   failed request, UID, generation, RV `401`, history, release and private
   receipts were unchanged. At **21:19:59Z**, spec/UID/generation/history and
   receipts/upgrade/gates still matched the saved baseline. RV later became
   `402` with requirements refresh time `21:19:53Z`, consistent with ordinary
   status refresh, not evidence of a successful assertion write. Guest 5.0.2
   remained Completed/Available=True/Failing=False/Progressing=False and the
   original worker UID remained Ready=True. The API Application was
   Synced/Healthy at deployed `d70d649...`; migration Job UID
   `daa14792-1593-4a92-abe8-2b8f32ba3fd1` and creation timestamp
   `2026-11-17T19:25:31Z` remained unchanged and Complete.
5. **Evidence recorded here.** Primary ownership journal checksum remains
   `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`;
   no primary `.updates.json` exists. No production code, chart, cloud IAM
   policy or resource was changed to bypass the blocked check.

## Root cause, independently reproduced without network

The deployed source's `orlop/pkg/apiserver/handlers/converting.go`,
`ConvertingResourceHandler.preparePatch`, unconditionally unmarshals the
request into `map[string]interface{}` and then invokes `jsonMergePatch`.
It does **not** dispatch on `Content-Type`. An RFC 6902 JSONPatch array
therefore fails object decoding before any `test` op, schema validation or
store update. The private/non-converting `ResourceHandler` does support
JSONPatch; that support does not extend to this public route.

[`tools/api-guard-diagnosis/main.go`](tools/api-guard-diagnosis/main.go) is a
self-contained **offline** reproduction using the actual public converting
handler, public/private schemas/converter and an in-memory store. It makes no
network/auth calls and needs no protected journal. Run/vet passed using the
CLI/Gecko external workfile from the executor checkpoint; Gecko local
`367f546...` has **no apiserver-code diff** from deployed `d70d649...`.
The reproduction returned:

```text
HTTP 400
invalid patch JSON: json: cannot unmarshal array into Go value of type map[string]interface {}
```

The complete encoded response digest is
`sha256:87f8744aed85e4dbe6c278298c284f85ce91695adaacd3228684e70c7e8c6c22`,
**byte-identical to the live response digest**. This strongly establishes the
parsing failure for this request. It does not establish that UID/RV guards are
absent or insecure—**neither guard was reached or validated**. The offline
tool omits live authorization middleware/Spanner and is not a general server
emulator.

Reproduce offline from `/Users/asegundo/git/osd/gcp-hcp-ctl`:

```sh
GOWORK=/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-p3.go.work \
  go run /Users/asegundo/git/osd/gcp-hcp/experiments/iam-bootstrap/wifconfigtemplate-evidence/tools/api-guard-diagnosis/main.go
```

The earlier local negative API tests accepted any 4xx, so an unrelated parsing
400 also passed them. Their result must not be used as proof that JSONPatch
preconditions work. The executor's direct HTTP unit tests verify what it sends
but use a fake server; they likewise do not prove deployed protocol support.
The architect reviewed and approved this diagnosis, stop decision and archive;
there was no approval to change production code or redeploy in this step.

## Next scoped work — not executed

Agree on and implement the compatible public API patch contract (likely
proper JSONPatch support), preserving status isolation, authorization,
schema/defaulting, metadata identity and optimistic storage checks. Add a
**successful valid JSONPatch** regression plus specific wrong-UID/stale-RV
failure checks, rather than generic 4xx assertions. Review/test/commit/build,
then separately approve any dev rollout with the migration-Job pin preserved.
Only afterward repeat the bounded live guard rehearsal under fresh state.

Do not silently switch the CLI to merge patch, omit UID tests, enable gates,
append a record, or allow cloud updates followed by a known-unrecordable
assertion. Full live IAM update and blocked/released upgrade acceptance remain
incomplete and require their existing target/permission/timing approvals.
