package deploy

import (
	"os"
	"strings"
	"testing"
)

const multiAddressCaddyfile = `
airbg.org, kanarche.eu {
	tls /a /b {
		client_auth {
			mode require_and_verify
		}
	}
}

www.airbg.org, www.kanarche.eu {
	redir https://airbg.org{uri} permanent
}
`

func TestSiteBlockCanListSeveralAddresses(t *testing.T) {
	blocks, err := parseCaddyBlocks("test", multiAddressCaddyfile)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"airbg.org", "kanarche.eu"} {
		if !strings.Contains(blocks[name], "require_and_verify") {
			t.Errorf("lookup by %s did not find the shared block; got %v", name, keysOf(blocks))
		}
	}
	for _, name := range []string{"www.airbg.org", "www.kanarche.eu"} {
		if !strings.Contains(blocks[name], "redir") {
			t.Errorf("lookup by %s did not find the shared www block", name)
		}
	}
}

// Mutation proof: a name whose block lacks client_auth must be reported, so
// deleting client_auth from the kanarche.eu address fails the suite.
func TestClientAuthInvariantFailsWhenAnAddressLosesIt(t *testing.T) {
	mutated := strings.Replace(multiAddressCaddyfile, "airbg.org, kanarche.eu {", "airbg.org {", 1) +
		"\nkanarche.eu {\n\treverse_proxy app:8080\n}\n"
	blocks, err := parseCaddyBlocks("test", mutated)
	if err != nil {
		t.Fatal(err)
	}
	if got := clientAuthProblems("airbg.org", blocks["airbg.org"]); len(got) != 0 {
		t.Errorf("airbg.org flagged unexpectedly: %v", got)
	}
	if got := clientAuthProblems("kanarche.eu", blocks["kanarche.eu"]); len(got) == 0 {
		t.Error("kanarche.eu without client_auth was not flagged")
	}
}

// The committed Caddyfile's proxied names must all pass the same check, so a
// future multi-address header cannot quietly drop one.
func TestCommittedProxiedNamesRequireClientAuth(t *testing.T) {
	data, err := os.ReadFile("Caddyfile")
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := parseCaddyBlocks("Caddyfile", string(data))
	if err != nil {
		t.Fatal(err)
	}
	for name, block := range blocks {
		if strings.HasPrefix(name, "tiles.") {
			continue
		}
		if p := clientAuthProblems(name, block); len(p) != 0 {
			t.Errorf("%v", p)
		}
	}
}

// The privacy page promises no stored IPs. Caddy's default logger writes error
// lines carrying them, so the global default log must delete these fields.
func TestCaddyfileDefaultLogStripsClientData(t *testing.T) {
	data, err := os.ReadFile("Caddyfile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "\n{\n")
	end := strings.Index(src, "\n}\n")
	if start < 0 || end < start {
		t.Fatal("no global options block in Caddyfile")
	}
	global := src[start:end]
	if !strings.Contains(global, "log default") {
		t.Fatal("global block has no `log default`")
	}
	if !strings.Contains(global, "format filter") || !strings.Contains(global, "wrap json") {
		t.Error("default log must use `format filter` wrapping json")
	}
	for _, f := range []string{"request>remote_ip", "request>client_ip", "request>headers", "remote_ip", "client_ip"} {
		found := false
		for _, line := range strings.Split(global, "\n") {
			if fs := strings.Fields(line); len(fs) == 2 && fs[0] == f && fs[1] == "delete" {
				found = true
			}
		}
		if !found {
			t.Errorf("default log does not delete %s", f)
		}
	}
}
