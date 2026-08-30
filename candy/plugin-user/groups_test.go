package user

import (
	"context"
	"strings"
	"testing"

	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/spec"
)

// groupsExec reuses the repo's existing multiExec (the linger leg's helper) rather than
// declaring a second one — the group assertions need exactly the same shape: getent plus
// one more probe in a single RunVerb.
func groupsExec(groupLine string, groupExit int) *multiExec {
	return &multiExec{replies: map[string]struct {
		out  string
		exit int
	}{
		"getent passwd": {"user:x:1000:1000::/home/user:/bin/bash\n", 0},
		"id -nG":        {groupLine, groupExit},
	}}
}

func ccWithGroups(groupLine string) *fakeCC {
	return &fakeCC{exec: groupsExec(groupLine, 0)}
}

func runUser(cc kit.CheckContext, in map[string]any) kit.Result {
	return verb{}.RunVerb(context.Background(), cc, &spec.Op{PluginInput: in})
}

func TestUserGroups_MemberPasses(t *testing.T) {
	res := runUser(ccWithGroups("user wheel audio\n"),
		map[string]any{"user": "user", "groups": []any{"wheel"}})
	if res.Status != kit.StatusPass {
		t.Errorf("expected pass for a wheel member, got %+v", res)
	}
}

func TestUserGroups_NonMemberFails(t *testing.T) {
	res := runUser(ccWithGroups("user audio video\n"),
		map[string]any{"user": "user", "groups": []any{"wheel"}})
	if res.Status != kit.StatusFail {
		t.Fatalf("expected fail for a non-member, got %+v", res)
	}
	// The message must name the groups the account DOES have — otherwise the reader's
	// next step is another round-trip to the guest to find out.
	if !strings.Contains(res.Message, "audio") {
		t.Errorf("the failure must report the actual group list; got %q", res.Message)
	}
}

// THE substring trap, asserted directly. A `grep -w docker` or a `contains:` matcher over
// the joined group list matches `docker-users`, and this field exists precisely so that a
// security posture cannot pass by near-miss.
func TestUserGroups_SimilarNameIsNotAMatch(t *testing.T) {
	res := runUser(ccWithGroups("user docker-users wheelie\n"),
		map[string]any{"user": "user", "groups": []any{"wheel"}})
	if res.Status != kit.StatusFail {
		t.Errorf("`wheelie` must not satisfy membership of `wheel`, got %+v", res)
	}

	// And the same in the negative direction: being in `docker-users` must NOT trip a
	// not_groups: [docker] assertion, or every hardened image reports a false breach.
	res = runUser(ccWithGroups("user docker-users\n"),
		map[string]any{"user": "user", "not_groups": []any{"docker"}})
	if res.Status != kit.StatusPass {
		t.Errorf("`docker-users` must not count as membership of `docker`, got %+v", res)
	}
}

// The security assertion this field exists for: the docker group is root-equivalent.
func TestUserGroups_NotGroupsCatchesRealMembership(t *testing.T) {
	res := runUser(ccWithGroups("user wheel docker\n"),
		map[string]any{"user": "user", "not_groups": []any{"docker"}})
	if res.Status != kit.StatusFail {
		t.Fatalf("a real docker membership must fail not_groups, got %+v", res)
	}
	if !strings.Contains(res.Message, "docker") {
		t.Errorf("the failure must name the forbidden group; got %q", res.Message)
	}
}

func TestUserGroups_BothDirectionsInOneStep(t *testing.T) {
	res := runUser(ccWithGroups("user wheel audio\n"), map[string]any{
		"user": "user", "groups": []any{"wheel"}, "not_groups": []any{"docker"},
	})
	if res.Status != kit.StatusPass {
		t.Errorf("wheel present + docker absent must pass, got %+v", res)
	}
}

// NEGATIVE CONTROL on the probe itself: with neither field set, `id -nG` must not run at
// all. Probing unconditionally would add a round trip to every existing `user:` step, and
// would fail on any venue where the account has no resolvable groups.
func TestUserGroups_NotProbedWhenUndeclared(t *testing.T) {
	// No `id -nG` reply is registered: if the verb probes anyway it gets exit 127 and
	// the step fails, which is exactly what this asserts must not happen. Same technique
	// as TestUserVerb_LingerNotProbedWhenUnset.
	cc := &fakeCC{exec: &fakeExec{matchPrefix: "getent passwd 'user'",
		stdout: "user:x:1000:1000::/home/user:/bin/bash\n", exit: 0}}
	res := verb{}.RunVerb(context.Background(), cc,
		&spec.Op{PluginInput: map[string]any{"user": "user"}})
	if res.Status != kit.StatusPass {
		t.Fatalf("a plain user: step must still pass without probing groups, got %+v", res)
	}
}

// A failing `id -nG` must FAIL, never silently report an empty group set — an empty set
// satisfies every not_groups assertion at once, which is the check failing open.
func TestUserGroups_ProbeFailureIsNotAnEmptySet(t *testing.T) {
	cc := &fakeCC{exec: groupsExec("", 1)}
	res := runUser(cc, map[string]any{"user": "user", "not_groups": []any{"docker"}})
	if res.Status != kit.StatusFail {
		t.Errorf("a failing id -nG must fail the check, not pass it vacuously; got %+v", res)
	}
}
