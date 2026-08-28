package user

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/spec"
)

// fakeExec is a kit.Executor returning canned RunCapture output (the getent passwd probe).
type fakeExec struct {
	matchPrefix, stdout string
	exit                int
}

func (f *fakeExec) RunCapture(_ context.Context, cmd string) (string, string, int, error) {
	if strings.HasPrefix(cmd, f.matchPrefix) || strings.Contains(cmd, f.matchPrefix) {
		return f.stdout, "", f.exit, nil
	}
	return "", "no fake response for: " + cmd, 127, nil
}
func (f *fakeExec) Kind() string { return "container" }

// fakeCC is a fake kit.CheckContext exercising the user verb's Exec leg.
type fakeCC struct{ exec kit.Executor }

func (c *fakeCC) Exec() kit.Executor { return c.exec }
func (c *fakeCC) Mode() kit.RunMode  { return kit.ModeLive }
func (c *fakeCC) HTTPDo(context.Context, kit.HTTPRequest) (kit.HTTPResponse, error) {
	return kit.HTTPResponse{}, nil
}
func (c *fakeCC) ResolveEndpoint(context.Context, int) (string, error) { return "", nil }
func (c *fakeCC) ResolveGraphicsEndpoint(context.Context, string) (kit.GraphicsEndpoint, error) {
	return kit.GraphicsEndpoint{}, nil
}
func (c *fakeCC) ResolveImageLabel(context.Context, string) (string, error) { return "", nil }
func (c *fakeCC) DialTimeout() time.Duration                                { return 3 * time.Second }
func (c *fakeCC) Box() string                                               { return "" }
func (c *fakeCC) Instance() string                                          { return "" }
func (c *fakeCC) Distros() []string                                         { return nil }
func (c *fakeCC) AddBackground(int)                                         {}

// TestUserVerb / TestUserVerb_UIDMismatch: getent output parsing. Relocated from
// charly/checkrun_verbs_test.go's TestRunner_User / TestRunner_User_UIDMismatch (#55
// decoupling cone, Batch D).
func TestUserVerb(t *testing.T) {
	cc := &fakeCC{exec: &fakeExec{matchPrefix: "getent passwd 'alice'", stdout: "alice:x:1000:1000:Alice:/home/alice:/bin/bash\n", exit: 0}}
	res := verb{}.RunVerb(context.Background(), cc, &spec.Op{PluginInput: map[string]any{"user": "alice", "uid": 1000, "home": "/home/alice"}})
	if res.Status != kit.StatusPass {
		t.Errorf("expected pass, got %+v", res)
	}
}

func TestUserVerb_UIDMismatch(t *testing.T) {
	cc := &fakeCC{exec: &fakeExec{matchPrefix: "getent passwd 'alice'", stdout: "alice:x:1500:1500:Alice:/home/alice:/bin/bash\n", exit: 0}}
	res := verb{}.RunVerb(context.Background(), cc, &spec.Op{PluginInput: map[string]any{"user": "alice", "uid": 1000}})
	if res.Status != kit.StatusFail {
		t.Errorf("expected fail, got %+v", res)
	}
}

// TestUserVerb_RenderProvisionScript: the ACT role renders an idempotent
// `id || useradd` with the given uid/home/shell. Relocated from
// charly/plugin_user_relocated_test.go's TestRelocatedUserVerb_DispatchesViaKit (the
// act-role behavior half; the dispatch wiring stays in charly).
func TestUserVerb_RenderProvisionScript(t *testing.T) {
	script, ok := verb{}.RenderProvisionScript(
		&spec.Op{PluginInput: map[string]any{"user": "svc", "uid": 1500, "home": "/home/svc", "shell": "/bin/sh"}}, nil)
	if !ok || !strings.Contains(script, "useradd") || !strings.Contains(script, "svc") {
		t.Fatalf("act: want a useradd script, got ok=%v %q", ok, script)
	}
}

// multiExec answers several commands, which the linger leg needs: the verb runs the
// getent probe AND a loginctl probe in one RunVerb.
type multiExec struct {
	replies map[string]struct {
		out  string
		exit int
	}
}

func (m *multiExec) RunCapture(_ context.Context, cmd string) (string, string, int, error) {
	for prefix, r := range m.replies {
		if strings.Contains(cmd, prefix) {
			return r.out, "", r.exit, nil
		}
	}
	return "", "no fake response for: " + cmd, 127, nil
}
func (m *multiExec) Kind() string { return "container" }

func lingerExec(lingerOut string, lingerExit int) *multiExec {
	return &multiExec{replies: map[string]struct {
		out  string
		exit int
	}{
		"getent passwd":      {"alice:x:1000:1000:Alice:/home/alice:/bin/bash\n", 0},
		"loginctl show-user": {lingerOut, lingerExit},
	}}
}

// linger is asserted ONLY when declared. Probing unconditionally would fail every
// container venue, where there is no logind to ask — so an entry that does not mention
// linger must not gain a second probe.
func TestUserVerb_LingerNotProbedWhenUnset(t *testing.T) {
	// No loginctl reply is registered: if the verb probes anyway it gets exit 127.
	cc := &fakeCC{exec: &fakeExec{matchPrefix: "getent passwd 'alice'",
		stdout: "alice:x:1000:1000:Alice:/home/alice:/bin/bash\n", exit: 0}}
	res := verb{}.RunVerb(context.Background(), cc, &spec.Op{
		PluginInput: map[string]any{"user": "alice"}})
	if res.Status != kit.StatusPass {
		t.Errorf("an entry that declares no linger was probed for it anyway: %+v", res)
	}
}

func TestUserVerb_LingerAssert(t *testing.T) {
	for _, tc := range []struct {
		name   string
		out    string
		exit   int
		want   bool
		status kit.Status
	}{
		{"enabled and wanted", "yes\n", 0, true, kit.StatusPass},
		{"disabled and not wanted", "no\n", 0, false, kit.StatusPass},
		{"disabled but wanted", "no\n", 0, true, kit.StatusFail},
		{"enabled but not wanted", "yes\n", 0, false, kit.StatusFail},
		// No logind user object is NOT the same as linger being off; reporting it as
		// "no" would be a lie that sends the reader to the wrong fix.
		{"loginctl cannot read the user", "", 1, true, kit.StatusFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cc := &fakeCC{exec: lingerExec(tc.out, tc.exit)}
			res := verb{}.RunVerb(context.Background(), cc, &spec.Op{
				PluginInput: map[string]any{"user": "alice", "linger": tc.want}})
			if res.Status != tc.status {
				t.Errorf("status = %v, want %v (%s)", res.Status, tc.status, res.Message)
			}
		})
	}
}

// The act renders enable-linger / disable-linger, and guards the loginctl PRESENCE so
// the failure names what was asked for rather than a missing binary.
func TestUserVerb_RenderProvisionScript_Linger(t *testing.T) {
	on, _ := verb{}.RenderProvisionScript(&spec.Op{
		PluginInput: map[string]any{"user": "alice", "linger": true}}, nil)
	if !strings.Contains(on, "loginctl enable-linger 'alice'") {
		t.Errorf("act does not enable linger: %s", on)
	}
	if !strings.Contains(on, "command -v loginctl") || !strings.Contains(on, "systemd-logind") {
		t.Errorf("act does not name the cause when loginctl is absent: %s", on)
	}
	// The account must still be created; linger is additive, not a replacement.
	if !strings.Contains(on, "useradd") {
		t.Errorf("act lost the useradd: %s", on)
	}

	off, _ := verb{}.RenderProvisionScript(&spec.Op{
		PluginInput: map[string]any{"user": "alice", "linger": false}}, nil)
	if !strings.Contains(off, "loginctl disable-linger 'alice'") {
		t.Errorf("linger: false does not disable: %s", off)
	}

	// Unset must render exactly as before — every existing user: step is unchanged.
	plain, _ := verb{}.RenderProvisionScript(&spec.Op{
		PluginInput: map[string]any{"user": "alice"}}, nil)
	if strings.Contains(plain, "loginctl") {
		t.Errorf("an entry declaring no linger gained a loginctl call: %s", plain)
	}
}
