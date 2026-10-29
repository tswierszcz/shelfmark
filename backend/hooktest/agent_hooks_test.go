// Package hooktest proves the Claude Code agent hooks in .claude/hooks against
// a throwaway git fixture: every case pipes a hook payload into the real script
// and asserts the exit code and the channel Claude reads (stderr on exit 2).
package hooktest

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	cleanGo     = "package pkg\n\n// Answer returns 42.\nfunc Answer() int { return 42 }\n"
	cleanGoTest = "package pkg\n\nimport \"testing\"\n\nfunc TestAnswer(t *testing.T) {\n\tif Answer() != 42 {\n\t\tt.Fatal(\"wrong answer\")\n\t}\n}\n"
	cleanTS     = "export const answer: number = 42;\n"
)

// npxStub stands in for the web toolchain: typegen creates the generated
// directory, tsc reports TS2322 for every app file containing BROKEN, vitest
// fails every app file containing FAILING_TEST, and STUB_TSC=missing /
// STUB_VITEST=missing make npx fail the way a missing local binary does.
const npxStub = `#!/usr/bin/env bash
if [ "$STUB_TSC" = "missing" ]; then echo "npm error could not determine executable to run" >&2; exit 1; fi
if [ "$STUB_TSC" = "missing-tsc" ] && [[ "$*" == *tsc* ]]; then echo "npm error could not determine executable to run" >&2; exit 1; fi
if [ "$STUB_VITEST" = "missing" ] && [[ "$*" == *vitest* ]]; then echo "npm error could not determine executable to run" >&2; exit 1; fi
case "$*" in
  *typegen*) mkdir -p .react-router/types; exit 0 ;;
  *vitest*)
    rc=0
    for f in $(grep -rl FAILING_TEST app 2>/dev/null); do echo " FAIL  $f > answer"; rc=1; done
    exit $rc ;;
  *tsc*)
    rc=0
    for f in $(grep -rl BROKEN app 2>/dev/null); do echo "$f(1,1): error TS2322: Type 'string' is not assignable to type 'number'."; rc=2; done
    exit $rc ;;
esac
exit 0
`

type fixture struct {
	t       *testing.T
	root    string
	hooks   string
	stubBin string
}

func hookDir(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return filepath.Join(strings.TrimSpace(string(out)), ".claude", "hooks")
}

func requireTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"bash", "jq", "git", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: agent hook tests need it", tool)
		}
	}
}

// newFixture creates a git repo shaped like this one (backend/ Go module,
// web/ TS project, .claude/hooks copied in) with a clean initial commit.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	requireTools(t)
	f := &fixture{t: t, root: t.TempDir(), stubBin: t.TempDir()}
	f.root, _ = filepath.EvalSymlinks(f.root)
	f.write("backend/go.mod", "module example.com/fixture\n\ngo 1.22\n")
	f.write("backend/pkg/answer.go", cleanGo)
	f.write("backend/pkg/answer_test.go", cleanGoTest)
	f.write("backend/internal/catalog/catalog.go", "package catalog\n\n// Name is the package name.\nconst Name = \"catalog\"\n")
	f.write("web/app/answer.ts", cleanTS)
	f.write("web/vitest.config.ts", "export default {};\n")
	f.write("web/.gitignore", "/.react-router/\n")
	f.write("README.md", "fixture\n")
	f.hooks = filepath.Join(f.root, ".claude", "hooks")
	for _, name := range []string{"check-edited-file.sh", "end-of-turn.sh"} {
		src, err := os.ReadFile(filepath.Join(hookDir(t), name))
		if err != nil {
			t.Fatalf("read hook %s: %v", name, err)
		}
		f.writeMode(".claude/hooks/"+name, string(src), 0o755)
	}
	settings, err := os.ReadFile(filepath.Join(hookDir(t), "..", "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	f.write(".claude/settings.json", string(settings))
	if err := os.WriteFile(filepath.Join(f.stubBin, "npx"), []byte(npxStub), 0o755); err != nil {
		t.Fatal(err)
	}
	f.git("init", "-q", "-b", "main")
	f.git("add", "-A")
	f.git("-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "init")
	return f
}

func (f *fixture) write(rel, content string) { f.writeMode(rel, content, 0o644) }

func (f *fixture) writeMode(rel, content string, mode os.FileMode) {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

type result struct {
	code   int
	stdout string
	stderr string
}

// run pipes payload into script with cwd as the working directory. extraEnv
// entries override the inherited environment (PATH gets the npx stub first).
func (f *fixture) run(script, cwd, payload string, extraEnv ...string) result {
	f.t.Helper()
	cmd := exec.Command("bash", script)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(payload)
	cmd.Env = append(os.Environ(),
		"PATH="+f.stubBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMPDIR="+f.t.TempDir(),
		"GOTOOLCHAIN=local",
		"GOFLAGS=",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		f.t.Fatalf("run %s: %v", script, err)
	}
	return result{code, stdout.String(), stderr.String()}
}

func editPayload(cwd, file string) string {
	b, _ := json.Marshal(map[string]any{
		"session_id": "s1", "cwd": cwd, "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": file},
	})
	return string(b)
}

func stopPayload(cwd string, active bool) string {
	b, _ := json.Marshal(map[string]any{"session_id": "s1", "cwd": cwd, "stop_hook_active": active})
	return string(b)
}

func (f *fixture) perEdit(file string, extraEnv ...string) result {
	return f.run(filepath.Join(f.hooks, "check-edited-file.sh"), f.root, editPayload(f.root, file), extraEnv...)
}

func (f *fixture) stop(active bool, extraEnv ...string) result {
	return f.run(filepath.Join(f.hooks, "end-of-turn.sh"), f.root, stopPayload(f.root, active), extraEnv...)
}

func wantBlock(t *testing.T, r result, mention ...string) {
	t.Helper()
	if r.code != 2 {
		t.Fatalf("exit = %d, want 2 (blocking)\nstderr:\n%s\nstdout:\n%s", r.code, r.stderr, r.stdout)
	}
	for _, m := range mention {
		if !strings.Contains(r.stderr, m) {
			t.Fatalf("stderr does not mention %q:\n%s", m, r.stderr)
		}
	}
}

func wantPass(t *testing.T, r result) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr:\n%s\nstdout:\n%s", r.code, r.stderr, r.stdout)
	}
}

func TestPerEdit_BrokenFilesBlock(t *testing.T) {
	f := newFixture(t)

	f.write("backend/pkg/answer.go", "package pkg\n\nfunc Answer() int {\n\tunused := 1\n\treturn 42\n}\n")
	wantBlock(t, f.perEdit(filepath.Join(f.root, "backend/pkg/answer.go")), "go vet", "backend/pkg/answer.go", "declared and not used")

	f.write("backend/pkg/answer.go", "package pkg\nfunc Answer() int {return 42}\n")
	wantBlock(t, f.perEdit(filepath.Join(f.root, "backend/pkg/answer.go")), "gofmt", "backend/pkg/answer.go")

	f.write("web/app/answer.ts", "export const answer: number = 'BROKEN';\n")
	wantBlock(t, f.perEdit(filepath.Join(f.root, "web/app/answer.ts")), "web/app/answer.ts", "TS2322")
}

func TestPerEdit_PassesOrSkips(t *testing.T) {
	f := newFixture(t)
	cases := map[string]string{
		"clean go":      editPayload(f.root, filepath.Join(f.root, "backend/pkg/answer.go")),
		"clean ts":      editPayload(f.root, filepath.Join(f.root, "web/app/answer.ts")),
		"skipped type":  editPayload(f.root, filepath.Join(f.root, "README.md")),
		"missing file":  editPayload(f.root, filepath.Join(f.root, "backend/pkg/gone.go")),
		"empty object":  "{}",
		"not json":      "not json",
		"no file path":  `{"cwd":"` + f.root + `","tool_name":"Edit","tool_input":{}}`,
		"empty payload": "",
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			wantPass(t, f.run(filepath.Join(f.hooks, "check-edited-file.sh"), f.root, payload))
		})
	}
}

func TestStop_SweepsChangesWithOneRetry(t *testing.T) {
	f := newFixture(t)

	t.Run("no changes passes", func(t *testing.T) {
		wantPass(t, f.stop(false))
	})

	// Written straight to disk (as a shell rewrite would), never seen by the per-edit hook.
	f.write("backend/pkg/answer.go", "package pkg\n\nfunc Answer() int {\n\tunused := 1\n\treturn 42\n}\n")
	t.Run("shell-written go error blocks", func(t *testing.T) {
		wantBlock(t, f.stop(false), "Go checks fail", "answer.go")
	})
	t.Run("retry flag lets the turn end", func(t *testing.T) {
		wantPass(t, f.stop(true))
	})
	t.Run("cursor loop_count lets the turn end", func(t *testing.T) {
		wantPass(t, f.run(filepath.Join(f.hooks, "end-of-turn.sh"), f.root, `{"cwd":"`+f.root+`","loop_count":1}`))
	})

	f.write("backend/pkg/answer.go", cleanGo)
	f.write("backend/pkg/answer_test.go", strings.Replace(cleanGoTest, "!= 42", "!= 41", 1))
	t.Run("failing test blocks", func(t *testing.T) {
		wantBlock(t, f.stop(false), "go test", "wrong answer")
	})

	f.write("backend/pkg/answer_test.go", cleanGoTest)
	f.write("web/app/answer.ts", "export const answer: number = 'BROKEN';\n")
	t.Run("web type error blocks", func(t *testing.T) {
		wantBlock(t, f.stop(false), "Web typecheck fails", "app/answer.ts")
	})

	f.write("web/app/answer.test.ts", "// FAILING_TEST\n")
	t.Run("type error stops web tests and says so", func(t *testing.T) {
		r := f.stop(false)
		wantBlock(t, r, "Web typecheck fails", "Web tests not run")
		if strings.Contains(r.stderr, "Web tests fail") {
			t.Fatalf("vitest ran although tsc failed:\n%s", r.stderr)
		}
	})

	f.write("web/app/answer.ts", cleanTS)
	f.write("web/app/answer.test.ts", "// FAILING_TEST\n")
	t.Run("failing web test blocks", func(t *testing.T) {
		wantBlock(t, f.stop(false), "Web tests fail", "app/answer.test.ts")
	})
	t.Run("missing vitest is reported, not silent", func(t *testing.T) {
		r := f.stop(false, "STUB_VITEST=missing")
		wantPass(t, r)
		if !strings.Contains(r.stdout, "web tests (vitest not installed") {
			t.Fatalf("expected a systemMessage naming the skipped web tests, got stdout:\n%s", r.stdout)
		}
	})

	f.write("web/app/answer.test.ts", "// passes\n")
	f.write("backend/internal/catalog/catalog.go", "package catalog\n\n// Name is the package name.\nconst Name = \"catalog!\"\n")
	t.Run("catalog skip is reported, not silent", func(t *testing.T) {
		r := f.stop(false)
		wantPass(t, r)
		if !strings.Contains(r.stdout, "skipped: internal/catalog tests") {
			t.Fatalf("expected a systemMessage naming the skipped catalog tests, got stdout:\n%s", r.stdout)
		}
	})
}

func TestCheckoutResolution(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	other.write("backend/pkg/answer.go", "package pkg\nfunc Answer() int {return 42}\n")

	t.Run("stale CLAUDE_PROJECT_DIR is ignored", func(t *testing.T) {
		wantPass(t, f.perEdit(filepath.Join(f.root, "backend/pkg/answer.go"), "CLAUDE_PROJECT_DIR="+other.root))
		wantPass(t, f.stop(false, "CLAUDE_PROJECT_DIR="+other.root))
	})

	t.Run("relative path resolves against cwd", func(t *testing.T) {
		f.write("backend/pkg/answer.go", "package pkg\nfunc Answer() int {return 42}\n")
		defer f.write("backend/pkg/answer.go", cleanGo)
		wantBlock(t, f.run(filepath.Join(f.hooks, "check-edited-file.sh"), f.root, editPayload(f.root, "backend/pkg/answer.go")), "gofmt")
	})

	t.Run("file of another repository is skipped", func(t *testing.T) {
		wantPass(t, f.perEdit(filepath.Join(other.root, "backend/pkg/answer.go")))
	})

	t.Run("sibling worktree edit is swept at end of turn", func(t *testing.T) {
		wt := filepath.Join(t.TempDir(), "wt")
		f.git("worktree", "add", "-q", "-b", "side", wt)
		broken := filepath.Join(wt, "backend/pkg/answer.go")
		if err := os.WriteFile(broken, []byte("package pkg\n\nfunc Answer() int {\n\tunused := 1\n\treturn 42\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		tmp := t.TempDir()
		edit := f.run(filepath.Join(f.hooks, "check-edited-file.sh"), f.root, editPayload(f.root, broken), "TMPDIR="+tmp)
		wantBlock(t, edit, "declared and not used")
		wantBlock(t, f.run(filepath.Join(f.hooks, "end-of-turn.sh"), f.root, stopPayload(f.root, false), "TMPDIR="+tmp), "Go checks fail", wt)
	})

	t.Run("settings commands reach the scripts from a worktree", func(t *testing.T) {
		var settings struct {
			Hooks map[string][]struct {
				Hooks []struct{ Command string } `json:"hooks"`
			} `json:"hooks"`
		}
		raw, _ := os.ReadFile(filepath.Join(f.root, ".claude/settings.json"))
		if err := json.Unmarshal(raw, &settings); err != nil {
			t.Fatalf("settings.json: %v", err)
		}
		wt := filepath.Join(t.TempDir(), "wt2")
		f.git("worktree", "add", "-q", "-b", "side2", wt)
		n := 0
		for _, groups := range settings.Hooks {
			for _, g := range groups {
				for _, h := range g.Hooks {
					cmd := exec.Command("sh", "-c", h.Command)
					cmd.Dir = wt
					cmd.Stdin = strings.NewReader("{}")
					cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR=/nonexistent", "PATH="+f.stubBin+":"+os.Getenv("PATH"))
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("command %q from worktree: %v\n%s", h.Command, err, out)
					}
					n++
				}
			}
		}
		if n != 2 {
			t.Fatalf("expected 2 hook commands, ran %d", n)
		}
	})
}

func TestNoSilentSuccess(t *testing.T) {
	f := newFixture(t)

	t.Run("missing web toolchain blocks", func(t *testing.T) {
		wantBlock(t, f.perEdit(filepath.Join(f.root, "web/app/answer.ts"), "STUB_TSC=missing"), "typegen could not run")
	})

	t.Run("missing typechecker blocks", func(t *testing.T) {
		wantBlock(t, f.perEdit(filepath.Join(f.root, "web/app/answer.ts"), "STUB_TSC=missing-tsc"), "Typecheck could not run")
	})

	t.Run("missing go blocks", func(t *testing.T) {
		// PATH with only the tools the script needs, and no go.
		bin := t.TempDir()
		for _, tool := range []string{"bash", "jq", "git", "dirname", "tr", "mkdir", "cat", "sort", "grep", "head"} {
			p, err := exec.LookPath(tool)
			if err != nil {
				t.Skipf("%s not found", tool)
			}
			if err := os.Symlink(p, filepath.Join(bin, tool)); err != nil {
				t.Fatal(err)
			}
		}
		r := f.run(filepath.Join(f.hooks, "check-edited-file.sh"), f.root, editPayload(f.root, filepath.Join(f.root, "backend/pkg/answer.go")), "PATH="+bin)
		wantBlock(t, r, "go is not on PATH")
	})
}
