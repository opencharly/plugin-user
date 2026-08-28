// Package user is the importable, COMPILED-IN host-coupled `user` verb: a MULTI-ROLE
// state-provision verb. CHECK (kit.CheckVerbProvider): `getent passwd` via the live
// kit.CheckContext and compare uid/gid/home/shell. ACT (kit.ProvisionActor): render an
// idempotent `useradd`. Relocated out of charly's module (formerly
// charly/plugin/builtins/user + charly/plugin_user.go) onto the sdk/kit
// contract; COMPILED-IN-ONLY. No matchers — direct field comparison.
package user

import (
	"context"
	"embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/opencharly/plugin-user/candy/plugin-user/params"
	"github.com/opencharly/sdk"
	"github.com/opencharly/sdk/kit"
	pb "github.com/opencharly/spec/proto"
	"github.com/opencharly/spec/shellquote"
	"github.com/opencharly/spec/spec"
)

//go:embed schema/*.cue
var schemaFS embed.FS

// NewCheckVerb returns the user verb as a kit.CheckVerbProvider for compiled-in
// registration. Because verb also implements kit.ProvisionActor, charly registers the
// multi-role (check + act) adapter.
func NewCheckVerb() kit.CheckVerbProvider { return verb{} }

// NewMeta advertises verb:user (plugin_input #UserInput) + the embedded CUE schema, via
// sdk.NewMeta — the ONE meta both placements use (compiled-in registerCompiledCheckVerb reads
// it via Describe; cmd/serve serves it out-of-process), so a kit candy has the SAME
// NewCheckVerb()+NewMeta() shape as every pb-provider plugin (R3).
func NewMeta() pb.PluginMetaServer {
	return sdk.NewMeta("2026.176.2500",
		[]sdk.ProvidedCapability{{Class: "verb", Word: "user", InputDef: "#UserInput", Primary: "user"}},
		schemaFS)
}

type verb struct{}

func (verb) Reserved() string { return "user" }

// RunVerb (do:assert) runs the getent-passwd probe via the live CheckContext and compares
// uid/gid/home/shell. Mirrors the former r.runUser.
func (verb) RunVerb(ctx context.Context, cc kit.CheckContext, op *spec.Op) kit.Result {
	var in params.UserInput
	kit.DecodeInput(op.PluginInput, &in)
	probe := fmt.Sprintf(`getent passwd %s`, shellquote.ShellQuote(in.User))
	out, _, exit, err := cc.Exec().RunCapture(ctx, probe)
	if err != nil {
		return kit.Failf("probe: %v", err)
	}
	if exit != 0 {
		return kit.Fail("user not found")
	}
	// Fields: user:x:uid:gid:gecos:home:shell
	parts := strings.SplitN(strings.TrimSpace(out), ":", 7)
	if len(parts) < 7 {
		return kit.Failf("unexpected passwd line: %q", out)
	}
	uid, _ := strconv.Atoi(parts[2])
	gid, _ := strconv.Atoi(parts[3])
	home, shell := parts[5], parts[6]
	if in.UID != nil && uid != *in.UID {
		return kit.Failf("uid=%d, want %d", uid, *in.UID)
	}
	if in.GID != nil && gid != *in.GID {
		return kit.Failf("gid=%d, want %d", gid, *in.GID)
	}
	if in.Home != "" && home != in.Home {
		return kit.Failf("home=%s, want %s", home, in.Home)
	}
	if in.Shell != "" && shell != in.Shell {
		return kit.Failf("shell=%s, want %s", shell, in.Shell)
	}
	// Linger is asserted ONLY when the step declares it: probing unconditionally would
	// fail every container venue, where there is no logind to ask.
	if in.Linger != nil {
		q := fmt.Sprintf("loginctl show-user %s --property=Linger --value",
			shellquote.ShellQuote(in.User))
		out, _, exit, err := cc.Exec().RunCapture(ctx, q)
		if err != nil {
			return kit.Failf("linger probe: %v", err)
		}
		if exit != 0 {
			// A user with no session has no logind user object at all; that is not the
			// same as linger being off, and reporting it as "no" would be a lie.
			return kit.Failf("linger: loginctl could not read user %s (exit %d) — "+
				"no systemd-logind on this venue, or the account has never logged in",
				in.User, exit)
		}
		got := strings.EqualFold(strings.TrimSpace(out), "yes")
		if got != *in.Linger {
			return kit.Failf("linger=%v, want %v — a scope: user unit will %s",
				got, *in.Linger,
				map[bool]string{true: "keep running after logout when it should not",
					false: "die the moment the session ends"}[got])
		}
	}
	return kit.Passf("uid=%d gid=%d", uid, gid)
}

// RenderProvisionScript (do:act) renders an idempotent useradd. ok is always true — a
// user act always has a create form. distros are unused (account fields are
// distro-agnostic). Mirrors the former userVerb.RenderProvisionScript.
func (verb) RenderProvisionScript(op *spec.Op, _ []string) (string, bool) {
	var in params.UserInput
	kit.DecodeInput(op.PluginInput, &in)
	flags := ""
	if in.UID != nil {
		flags += fmt.Sprintf(" -u %d", *in.UID)
	}
	if in.Home != "" {
		flags += " -m -d " + shellquote.ShellQuote(in.Home)
	}
	if in.Shell != "" {
		flags += " -s " + shellquote.ShellQuote(in.Shell)
	}
	name := shellquote.ShellQuote(in.User)
	script := fmt.Sprintf("id %[1]s >/dev/null 2>&1 || useradd%[2]s %[1]s", name, flags)
	if in.Linger != nil {
		// enable-linger/disable-linger are both idempotent, so this needs no guard of
		// its own. The loginctl PRESENCE check does need one: without it the step fails
		// with a bare "loginctl: not found", which names the missing tool rather than
		// the thing the author asked for.
		sub := "enable-linger"
		if !*in.Linger {
			sub = "disable-linger"
		}
		script += fmt.Sprintf(
			"\nif ! command -v loginctl >/dev/null 2>&1; then "+
				"echo \"user %[1]s: linger was requested but loginctl is absent — this venue "+
				"has no systemd-logind, so a scope: user unit cannot survive logout here\" >&2; "+
				"exit 1; fi\nloginctl %[2]s %[1]s", name, sub)
	}
	return script, true
}
