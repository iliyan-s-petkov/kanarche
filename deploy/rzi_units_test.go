package deploy

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// sandboxFlags is internal/docparse/README.md, "Run flags", flag for flag.
// I1 is --network none. I2 is the rest. Each is matched as a whole token
// sequence so that "--cpus 0.5" cannot pass as "--cpus 0.50".
var sandboxFlags = []string{
	"--network none",
	"--read-only",
	"--tmpfs /tmp:size=16m,noexec",
	"--cap-drop ALL",
	"--security-opt no-new-privileges",
	"--user 65534:65534",
	"--memory 256m",
	"--memory-swap 256m",
	"--cpus 0.5",
	"--pids-limit 64",
}

// unitSections parses a systemd unit into section -> key -> values. Backslash
// continuations are joined first, as systemd does.
func unitSections(t *testing.T, path string) map[string]map[string][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v, want nil", path, err)
	}
	joined := strings.ReplaceAll(string(data), "\\\n", " ")
	out := map[string]map[string][]string{}
	section := ""
	for _, line := range strings.Split(joined, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			out[section] = map[string][]string{}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || section == "" {
			continue
		}
		out[section][k] = append(out[section][k], strings.TrimSpace(v))
	}
	return out
}

func extractExecStart(t *testing.T) []string {
	t.Helper()
	u := unitSections(t, "airbg-rzi-extract.service")
	exec := u["Service"]["ExecStart"]
	if len(exec) != 1 {
		t.Fatalf("ExecStart lines = %d, want 1", len(exec))
	}
	return strings.Fields(exec[0])
}

func hasSequence(args []string, seq string) bool {
	want := strings.Fields(seq)
	for i := 0; i+len(want) <= len(args); i++ {
		match := true
		for j := range want {
			if args[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// rziDirs reads the directories the app is configured to use, so the unit and
// the compose mounts cannot drift from airbg.yaml.
func rziDirs(t *testing.T) (inbox, outbox string) {
	t.Helper()
	data, err := os.ReadFile("../airbg.yaml")
	if err != nil {
		t.Fatalf("ReadFile(airbg.yaml) error = %v, want nil", err)
	}
	var doc struct {
		Sea struct {
			RZI struct {
				InboxDir  string `yaml:"inbox_dir"`
				OutboxDir string `yaml:"outbox_dir"`
			} `yaml:"rzi"`
		} `yaml:"sea"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Unmarshal(airbg.yaml) error = %v, want nil", err)
	}
	in, out := doc.Sea.RZI.InboxDir, doc.Sea.RZI.OutboxDir
	if in == "" || out == "" {
		t.Fatalf("airbg.yaml sea.rzi inbox_dir=%q outbox_dir=%q, want both set", in, out)
	}
	return in, out
}

// allowedTokens is every dash-prefixed token ExecStart may contain: the
// sandbox flags' names plus the run options that do not widen the sandbox.
// Anything else (--ipc, --userns, --volumes-from, --privileged, --flag=value
// spellings) fails.
func allowedTokens() map[string]bool {
	m := map[string]bool{"--rm": true, "-v": true, "--name": true, "--stop-timeout": true}
	for _, f := range sandboxFlags {
		m[strings.Fields(f)[0]] = true
	}
	return m
}

// I1 and I2: the sandbox flags, each exactly once, and nothing else.
func TestExtractUnitCarriesEverySandboxFlag(t *testing.T) {
	args := extractExecStart(t)
	if len(args) < 3 || args[0] != "/usr/bin/docker" || args[1] != "run" || args[2] != "--rm" {
		t.Fatalf("ExecStart starts %v, want /usr/bin/docker run --rm", args)
	}
	for _, f := range sandboxFlags {
		if !hasSequence(args, f) {
			t.Errorf("ExecStart lacks %q", f)
		}
	}
	allowed := allowedTokens()
	counts := map[string]int{}
	for _, a := range args[3:] {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		if !allowed[a] {
			t.Errorf("ExecStart has token %q outside the allowlist", a)
		}
		counts[a]++
	}
	for _, f := range sandboxFlags {
		name := strings.Fields(f)[0]
		if counts[name] != 1 {
			t.Errorf("flag %s appears %d times, want exactly once", name, counts[name])
		}
	}
	if !hasSequence(args, "--name airbg-rzi-extract") || counts["--stop-timeout"] != 1 {
		t.Errorf("ExecStart needs --name airbg-rzi-extract and one --stop-timeout")
	}
}

// The unit fails closed on a missing or non-digest image, reads exactly one
// env file, and clears a leftover container of its own name around the run.
func TestExtractUnitGuardsImageAndLeftovers(t *testing.T) {
	svc := unitSections(t, "airbg-rzi-extract.service")["Service"]
	if got := svc["EnvironmentFile"]; len(got) != 1 || got[0] != "/etc/airbg/rzi-extract.env" {
		t.Errorf("EnvironmentFile = %v, want exactly /etc/airbg/rzi-extract.env (no - prefix)", got)
	}
	guard, rmPre := false, false
	for _, p := range svc["ExecStartPre"] {
		if strings.Contains(p, "$DOCPARSE_IMAGE") && strings.Contains(p, "@sha256:") && strings.Contains(p, "exit 1") {
			guard = true
		}
		if p == "-/usr/bin/docker rm -f airbg-rzi-extract" {
			rmPre = true
		}
	}
	if !guard {
		t.Errorf("ExecStartPre lacks a DOCPARSE_IMAGE @sha256: guard that exits 1: %v", svc["ExecStartPre"])
	}
	if !rmPre {
		t.Errorf("ExecStartPre lacks -/usr/bin/docker rm -f airbg-rzi-extract: %v", svc["ExecStartPre"])
	}
	if got := svc["ExecStopPost"]; len(got) != 1 || got[0] != "-/usr/bin/docker rm -f airbg-rzi-extract" {
		t.Errorf("ExecStopPost = %v, want -/usr/bin/docker rm -f airbg-rzi-extract", got)
	}
}

// I3: the inbox is read-only in the sandbox, the outbox is not.
func TestExtractUnitMountsInboxReadOnly(t *testing.T) {
	inbox, outbox := rziDirs(t)
	args := extractExecStart(t)
	if !hasSequence(args, "-v "+inbox+":/in:ro") {
		t.Errorf("ExecStart lacks -v %s:/in:ro", inbox)
	}
	if !hasSequence(args, "-v "+outbox+":/out") {
		t.Errorf("ExecStart lacks -v %s:/out", outbox)
	}
	mounts := 0
	for _, a := range args {
		if a == "-v" || a == "--volume" || strings.HasPrefix(a, "--mount") {
			mounts++
		}
		if strings.HasPrefix(a, outbox+":") && strings.HasSuffix(a, ":ro") {
			t.Errorf("outbox mounted read-only (%s), the extractor writes there", a)
		}
	}
	if mounts != 2 {
		t.Errorf("ExecStart has %d mounts, want exactly 2 (inbox, outbox)", mounts)
	}
}

// The image is pinned: by digest, or by a variable that an env file with no
// secrets in it supplies. A literal tag such as :latest is neither.
func TestExtractUnitImageIsPinned(t *testing.T) {
	u := unitSections(t, "airbg-rzi-extract.service")
	args := extractExecStart(t)
	img := args[len(args)-1]
	switch {
	case regexp.MustCompile(`@sha256:[0-9a-f]{64}$`).MatchString(img):
	case img == "${DOCPARSE_IMAGE}":
		envFiles := u["Service"]["EnvironmentFile"]
		if len(envFiles) != 1 || strings.HasSuffix(envFiles[0], "/.env") {
			t.Errorf("EnvironmentFile = %v, want one file that is not the app .env", envFiles)
		}
	default:
		t.Errorf("image %q is not pinned by digest or ${DOCPARSE_IMAGE}", img)
	}
}

// The unit must not reach the docker socket, the network or secrets.
func TestExtractUnitHasNoReachBeyondTheSandbox(t *testing.T) {
	data, err := os.ReadFile("airbg-rzi-extract.service")
	if err != nil {
		t.Fatal(err)
	}
	var body []string
	for _, l := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "#") {
			body = append(body, l)
		}
	}
	text := strings.Join(body, "\n")
	for _, bad := range []string{"docker.sock", "--network host", "--privileged", "--env", " -e ", "pgpass", "--pid ", "--pid=", "--cap-add", "--device"} {
		if strings.Contains(text, bad) {
			t.Errorf("unit contains %q", bad)
		}
	}
}

func TestExtractUnitTimeoutIsBounded(t *testing.T) {
	u := unitSections(t, "airbg-rzi-extract.service")
	v := u["Service"]["TimeoutStartSec"]
	if len(v) != 1 {
		t.Fatalf("TimeoutStartSec lines = %d, want 1", len(v))
	}
	n, err := strconv.Atoi(strings.TrimSuffix(v[0], "s"))
	if err != nil || n <= 0 || n > 180 {
		t.Errorf("TimeoutStartSec = %q, want 1..180 seconds", v[0])
	}
	if got := u["Service"]["Type"]; len(got) != 1 || got[0] != "oneshot" {
		t.Errorf("Type = %v, want oneshot", got)
	}
}

func TestExtractTimerRunsEverySixHours(t *testing.T) {
	u := unitSections(t, "airbg-rzi-extract.timer")
	cal := u["Timer"]["OnCalendar"]
	if len(cal) != 1 || !regexp.MustCompile(`^\*-\*-\* 00/6:\d\d:\d\d$`).MatchString(cal[0]) {
		t.Errorf("OnCalendar = %v, want *-*-* 00/6:MM:SS", cal)
	}
	if v := u["Timer"]["RandomizedDelaySec"]; len(v) != 1 || v[0] == "0" {
		t.Errorf("RandomizedDelaySec = %v, want a non-zero value", v)
	}
	if v := u["Timer"]["Persistent"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("Persistent = %v, want true", v)
	}
	if got := u["Install"]["WantedBy"]; len(got) != 1 || got[0] != "timers.target" {
		t.Errorf("WantedBy = %v, want timers.target", got)
	}
}
