// One-role live rehearsal. Never operates on retained installation resources.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	gcpiam "github.com/openshift-online/gcp-hcp-ctl/pkg/gcp/iam"
	"github.com/openshift-online/gcp-hcp-ctl/pkg/infra/iam/application"
	"google.golang.org/api/googleapi"
	iam "google.golang.org/api/iam/v1"
)

const project = "apahim-dev-test-1"
const number = "35809355916"
const roleID = "wif4_etag_20261119a"
const roleName = "projects/" + project + "/roles/" + roleID
const title = "WIF Phase 4 isolated unbound etag rehearsal"
const stateDir = "/Users/asegundo/.local/state/gcp-hcp/wif-phase4-role-20261119"
const statePath = stateDir + "/role-state.json"

var initial = []string{"compute.instances.get"}
var union = []string{"compute.instances.get", "compute.zones.list"}
var lostResponse = errors.New("EXPECTED simulated response loss; run recover, never repeat update-lost")

type journal struct {
	Kind                  string    `json:"kind"`
	OperationID           string    `json:"operationID"`
	Role                  string    `json:"role"`
	Owner                 string    `json:"owner"`
	Phase                 string    `json:"phase"`
	InitialEtag           string    `json:"initialEtag,omitempty"`
	UpdatedEtag           string    `json:"updatedEtag,omitempty"`
	StaleEtagStatus       int       `json:"staleEtagStatus,omitempty"`
	PermissionUpdateCalls int       `json:"permissionUpdateCalls"`
	DeleteCalls           int       `json:"deleteCalls"`
	DeleteEtag            string    `json:"deleteEtag,omitempty"`
	ObservedAt            time.Time `json:"observedAt"`
}

func private(path string, directory bool) error {
	i, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if i.Mode()&os.ModeSymlink != 0 || directory != i.IsDir() || i.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("state path is not private or has changed type")
	}
	if !directory && !i.Mode().IsRegular() {
		return fmt.Errorf("state must be a regular file")
	}
	if s, ok := i.Sys().(*syscall.Stat_t); !ok || s.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("state ownership mismatch")
	}
	return nil
}

func save(j *journal, first bool) error {
	if err := private(stateDir, true); err != nil {
		return err
	}
	j.ObservedAt = time.Now().UTC()
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(stateDir, ".role-state-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(append(b, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if first {
		err = os.Link(f.Name(), statePath)
	} else {
		if err = private(statePath, false); err != nil {
			return err
		}
		err = os.Rename(f.Name(), statePath)
	}
	if err != nil {
		return err
	}
	d, err := os.Open(stateDir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func load() (*journal, error) {
	if err := private(statePath, false); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	var j journal
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	if j.Kind != "WIFRoleRehearsal/v1" || j.Role != roleName || len(j.OperationID) != 32 || j.Owner != "gcphcpctl/wif4-role-rehearsal/v1/"+j.OperationID {
		return nil, fmt.Errorf("unexpected role journal identity")
	}
	return &j, nil
}

func checkRole(r *iam.Role, j *journal, permissions []string) error {
	p := slices.Clone(r.IncludedPermissions)
	slices.Sort(p)
	if r.Name != roleName || r.Title != title || r.Description != j.Owner || r.Stage != "GA" || r.Deleted || r.Etag == "" || !slices.Equal(p, permissions) {
		return fmt.Errorf("role ownership, permissions, title, stage or etag differs")
	}
	return nil
}

func unbound(ctx context.Context, c *gcpiam.Client) error {
	p, err := c.GetProjectIAMPolicy(ctx)
	if err != nil {
		return err
	}
	for _, b := range p.Bindings {
		if b.Role == roleName && len(b.Members) > 0 {
			return fmt.Errorf("rehearsal role has unexpected project-policy binding; stop")
		}
	}
	return nil
}

func run(mode string) error {
	if mode == "help" {
		fmt.Println("preflight | create | update-lost | recover | stale-etag | cleanup | inspect; one fixed role only")
		return nil
	}
	for p := filepath.Dir(stateDir); p != filepath.Dir(p); p = filepath.Dir(p) {
		if i, err := os.Lstat(p); err != nil {
			return err
		} else if i.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("state parent is a symlink")
		}
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	if err := private(stateDir, true); err != nil {
		return err
	}
	lock, err := os.OpenFile(statePath+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := private(lock.Name(), false); err != nil {
		return err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another rehearsal holds the lock")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := gcpiam.NewClient(ctx, project)
	if err != nil {
		return err
	}
	g := &application.GCP{Client: c}
	n, err := g.ProjectNumber(ctx)
	if err != nil {
		return err
	}
	if n != number {
		return fmt.Errorf("customer project number mismatch")
	}
	if mode == "preflight" {
		if _, err := os.Lstat(statePath); !os.IsNotExist(err) {
			return fmt.Errorf("state already exists or unreadable; do not allocate replacement")
		}
		_, err = c.GetCustomRole(ctx, roleName)
		var api *googleapi.Error
		if !errors.As(err, &api) || api.Code != 404 {
			return fmt.Errorf("role ID is not independently absent; do not adopt or undelete")
		}
		if err := c.TestPermissions(ctx, []string{"iam.roles.create", "iam.roles.get", "iam.roles.update", "iam.roles.delete", "resourcemanager.projects.getIamPolicy"}); err != nil {
			return err
		}
		if err := c.CheckCustomPermissions(ctx, union); err != nil {
			return err
		}
		if err := unbound(ctx, c); err != nil {
			return err
		}
		fmt.Println(`{"roleAbsent":true,"projectNumber":"35809355916","authorityAndPermissionsChecked":true,"cloudWrites":0}`)
		return nil
	}
	if mode == "create" {
		if _, err := os.Lstat(statePath); !os.IsNotExist(err) {
			return fmt.Errorf("state exists; inspect it, do not recreate")
		}
		_, err = c.GetCustomRole(ctx, roleName)
		var api *googleapi.Error
		if !errors.As(err, &api) || api.Code != 404 {
			return fmt.Errorf("role ID collision or unproven absence")
		}
		if err := unbound(ctx, c); err != nil {
			return err
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return err
		}
		op := hex.EncodeToString(id[:])
		j := &journal{Kind: "WIFRoleRehearsal/v1", OperationID: op, Role: roleName, Owner: "gcphcpctl/wif4-role-rehearsal/v1/" + op, Phase: "CreateIntent"}
		if err := save(j, true); err != nil {
			return err
		}
		_, err = c.CreateCustomRole(ctx, roleID, &iam.Role{Title: title, Description: j.Owner, Stage: "GA", IncludedPermissions: initial})
		if err != nil {
			return err
		}
		r, err := c.GetCustomRole(ctx, roleName)
		if err != nil {
			return err
		}
		if err := checkRole(r, j, initial); err != nil {
			return err
		}
		if err := unbound(ctx, c); err != nil {
			return err
		}
		j.Phase = "Created"
		j.InitialEtag = r.Etag
		if err := save(j, false); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(j)
	}
	j, err := load()
	if err != nil {
		return err
	}
	r, err := c.GetCustomRole(ctx, roleName)
	if err != nil {
		return err
	}
	switch mode {
	case "inspect":
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"journal": j, "role": r})
	case "update-lost":
		if j.Phase != "Created" || j.PermissionUpdateCalls != 0 {
			return fmt.Errorf("only original Created role can be updated")
		}
		if err := checkRole(r, j, initial); err != nil {
			return err
		}
		if r.Etag != j.InitialEtag {
			return fmt.Errorf("initial etag changed")
		}
		if err := unbound(ctx, c); err != nil {
			return err
		}
		if err := g.CheckRoleUpdateAuthority(ctx); err != nil {
			return err
		}
		j.Phase = "UpdateIntent"
		j.PermissionUpdateCalls = 1
		if err := save(j, false); err != nil {
			return err
		}
		if err := g.UpdateRolePermissions(ctx, application.Resource{Kind: "Role", Name: roleName, UID: j.InitialEtag, Owner: j.Owner, Stage: "GA", Permissions: union}); err != nil {
			return err
		}
		// Deliberately discard success after the real RPC, then exit without
		// saving after-state. A separate process must recover by GET only.
		fmt.Println(`{"simulatedLostResponseAfterSuccessfulRPC":true,"journalPhase":"UpdateIntent","permissionUpdateCalls":1}`)
		return lostResponse
	case "recover":
		if j.Phase != "UpdateIntent" || j.PermissionUpdateCalls != 1 {
			return fmt.Errorf("no expected unknown role update to recover")
		}
		if err := checkRole(r, j, union); err != nil {
			return err
		}
		if r.Etag == j.InitialEtag {
			return fmt.Errorf("permission update did not advance etag")
		}
		if err := unbound(ctx, c); err != nil {
			return err
		}
		j.Phase = "Recovered"
		j.UpdatedEtag = r.Etag
		if err := save(j, false); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(j)
	case "stale-etag":
		if j.Phase != "Recovered" || j.StaleEtagStatus != 0 {
			return fmt.Errorf("stale etag test must run exactly once after recovery")
		}
		if err := checkRole(r, j, union); err != nil {
			return err
		}
		if r.Etag != j.UpdatedEtag {
			return fmt.Errorf("role changed before stale etag test")
		}
		j.Phase = "StaleEtagIntent"
		if err := save(j, false); err != nil {
			return err
		}
		// Same approved union, old etag. No extra permission can be granted
		// even if the backend were to ignore the concurrency precondition.
		_, err := c.UpdateCustomRolePermissions(ctx, roleName, j.InitialEtag, union)
		var api *googleapi.Error
		if !errors.As(err, &api) || (api.Code != 409 && api.Code != 412) {
			return fmt.Errorf("stale etag was not rejected with 409/412; investigate before cleanup")
		}
		fresh, err := c.GetCustomRole(ctx, roleName)
		if err != nil {
			return err
		}
		if err := checkRole(fresh, j, union); err != nil {
			return err
		}
		if fresh.Etag != j.UpdatedEtag {
			return fmt.Errorf("stale etag request unexpectedly changed role revision")
		}
		j.StaleEtagStatus = api.Code
		j.Phase = "StaleEtagRejected"
		if err := save(j, false); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(j)
	case "cleanup":
		if j.Phase == "DeleteIntent" && r.Deleted && r.Name == roleName && r.Description == j.Owner {
			if err := unbound(ctx, c); err != nil {
				return err
			}
			j.Phase = "Cleaned"
			if err := save(j, false); err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(j)
		}
		if j.DeleteCalls != 0 || j.Phase == "Cleaned" {
			return fmt.Errorf("prior delete outcome must be inspected, never blindly retried")
		}
		switch j.Phase {
		case "CreateIntent", "Created", "UpdateIntent", "Recovered", "StaleEtagIntent", "StaleEtagRejected":
		default:
			return fmt.Errorf("unexpected phase for owned cleanup")
		}
		// Failure cleanup is permitted only after exact ownership/read-only
		// permissions proof, never by a role prefix or unproven absence.
		if err := checkRole(r, j, union); err != nil {
			if err := checkRole(r, j, initial); err != nil {
				return err
			}
		}
		if err := unbound(ctx, c); err != nil {
			return err
		}
		j.DeleteEtag = r.Etag
		j.Phase = "DeleteIntent"
		j.DeleteCalls = 1
		if err := save(j, false); err != nil {
			return err
		}
		if err := c.DeleteCustomRole(ctx, roleName, r.Etag); err != nil {
			return err
		}
		fresh, err := c.GetCustomRole(ctx, roleName)
		if err != nil {
			return err
		}
		if fresh.Name != roleName || fresh.Description != j.Owner || !fresh.Deleted {
			return fmt.Errorf("role soft-deletion not independently observed")
		}
		if err := unbound(ctx, c); err != nil {
			return err
		}
		j.Phase = "Cleaned"
		if err := save(j, false); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(j)
	default:
		return fmt.Errorf("unsupported rehearsal mode")
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "one explicit mode is required")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		// Never print cloud provider prose, access tokens or raw HTTP bodies.
		var api *googleapi.Error
		if errors.As(err, &api) {
			fmt.Fprintf(os.Stderr, "rehearsal stopped: cloud HTTP %d; retain journal\n", api.Code)
		} else if errors.Is(err, lostResponse) {
			fmt.Fprintln(os.Stderr, lostResponse.Error())
		} else {
			fmt.Fprintln(os.Stderr, "rehearsal stopped; retain journal and inspect approved diagnostics; raw provider errors suppressed")
		}
		os.Exit(1)
	}
}
