//go:generate ./gen.py

package tomltest

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing/fstest"
	"time"

	"github.com/BurntSushi/toml"
)

type testType uint8

const (
	TypeValid testType = iota
	TypeEncoder
	TypeInvalid
)

const DefaultVersion = "1.0.0"

//go:embed tests/*
var embeddedTests embed.FS

// TestCases embedded in toml-test, rooted to the "test/" directory.
func TestCases() fs.FS {
	f, err := fs.Sub(embeddedTests, "tests")
	if err != nil {
		panic(err)
	}

	fsys := make(fstest.MapFS)
	err = fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		var data []byte
		if !d.IsDir() {
			data, err = fs.ReadFile(f, path)
			if err != nil {
				return err
			}
		}

		fsys[path] = &fstest.MapFile{
			Data:    data,
			Mode:    fi.Mode(),
			ModTime: fi.ModTime(),
			Sys:     fi.Sys(),
		}
		if strings.HasPrefix(path, "valid") {
			fsys["encoder"+path[5:]] = &fstest.MapFile{
				Data:    data,
				Mode:    fi.Mode(),
				ModTime: fi.ModTime(),
				Sys:     fi.Sys(),
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return fsys
}

// Runner runs a set of tests.
//
// The validity of the parameters is not checked extensively; the caller should
// verify this if need be. See ./cmd/toml-test for an example.
type Runner struct {
	Files         fs.FS  // Test files.
	Decoder       Parser // Send data to a parser.
	Encoder       Parser
	RunTests      []string          // Tests to run; run all if blank.
	SkipTests     []string          // Tests to skip.
	Version       string            // TOML version to run tests for.
	Parallel      int               // Number of tests to run in parallel
	Timeout       time.Duration     // Maximum time for parse.
	IntAsFloat    bool              // Int values have type=float.
	Errors        map[string]string // Expected errors list.
	SkipMustError bool              // Tests in SkipTests must fail. Useful for CI.
}

func NewRunner(r Runner) Runner {
	if r.Version == "" || r.Version == "1.0" {
		r.Version = "1.0.0"
	} else if r.Version == "1.1" {
		r.Version = "1.1.0"
	} else if r.Version == "latest" {
		r.Version = "1.1.0"
	}
	if r.Files == nil {
		r.Files = TestCases()
	}
	return r
}

// A Parser instance is used to call the TOML parser we test.
//
// By default this is done through an external command.
type Parser interface {
	// Run a parser command (decoder or encoder).
	//
	// If outputIsError is true then it's assumed that an encoding error
	// occurred.
	//
	// An error return should only be used in case an unrecoverable error
	// occurred; failing to encode to TOML is not an error, but the encoder
	// unexpectedly panicking is.
	Run(ctx context.Context, input string) (pid int, output string, outputIsError bool, err error)

	Cmd() []string
}

// Tests are tests to run.
type Tests struct {
	Tests []Test `json:"tests"`

	// Set when test are run.

	Skipped       int `json:"skipped"`
	PassedValid   int `json:"passed_valid"`
	FailedValid   int `json:"failed_valid"`
	PassedInvalid int `json:"passed_invalid"`
	FailedInvalid int `json:"failed_invalid"`
	PassedEncoder int `json:"passed_encoder"`
	FailedEncoder int `json:"failed_encoder"`
}

// Result is the result of a single test.
type Test struct {
	Path string `json:"path"` // Path of test, e.g. "valid/string-test"

	// Set when a test is run.

	Skipped          bool          `json:"skipped"`            // Skipped this test?
	Failure          string        `json:"failure"`            // Failure message.
	Key              string        `json:"key"`                // TOML key the failure occured on; may be blank.
	Input            string        `json:"input"`              // The test case that we sent to the external program.
	Output           string        `json:"output"`             // Output from the external program.
	Want             string        `json:"want"`               // The output we want.
	OutputFromStderr bool          `json:"output_from_stderr"` // The Output came from stderr, not stdout.
	PID              int           `json:"pid"`                // PID from test run.
	Timeout          time.Duration `json:"-"`                  // Maximum time for parse.
	IntAsFloat       bool          `json:"-"`                  // Int values have type=float.
}

type timeoutError struct{ d time.Duration }

func (err timeoutError) Error() string {
	return fmt.Sprintf("command timed out after %s; increase -timeout if this isn't an infinite loop or pathological behaviour", err.d)
}

// List all tests in Files for the current TOML version.
func (r Runner) List() ([]string, error) {
	if _, ok := versions[r.Version]; !ok {
		v := make([]string, 0, len(versions))
		for k := range versions {
			v = append(v, k)
		}
		sort.Strings(v)
		return nil, fmt.Errorf("tomltest.Runner.Run: unknown version: %q (supported: \"%s\")",
			r.Version, strings.Join(v, `", "`))
	}

	var (
		v       = versions[r.Version]
		exclude = make([]string, 0, 8)
	)
	for {
		for _, vv := range v.exclude {
			if strings.HasPrefix(vv, "valid") {
				v.exclude = append(v.exclude, "encoder"+vv[5:])
			}
		}
		exclude = append(exclude, v.exclude...)
		if v.inherit == "" {
			break
		}
		v = versions[v.inherit]
	}

	ls := make([]string, 0, 256)
	if err := r.findTOML("valid", &ls, exclude); err != nil {
		return nil, fmt.Errorf(`reading "valid/": %w`, err)
	}
	if err := r.findTOML("encoder", &ls, exclude); err != nil {
		return nil, fmt.Errorf(`reading "encoder/": %w`, err)
	}
	if err := r.findTOML("invalid", &ls, exclude); err != nil {
		return nil, fmt.Errorf(`reading "invalid/": %w`, err)
	}
	return ls, nil
}

// Run all tests listed in t.RunTests.
//
// TODO: give option to:
// - Run all tests with \n replaced with \r\n
// - Run all tests with EOL removed
// - Run all tests with '# comment' appended to every line.
func (r Runner) Run() (Tests, error) {
	skipped, err := r.findTests()
	if err != nil {
		return Tests{}, fmt.Errorf("tomltest.Runner.Run: %w", err)
	}
	if r.Parallel == 0 {
		r.Parallel = 1
	}
	if r.Timeout == 0 {
		r.Timeout = 1 * time.Second
	}
	if r.Errors == nil {
		r.Errors = make(map[string]string)
	}
	nerr := make(map[string]string)
	for k, v := range r.Errors {
		if !strings.HasPrefix(k, "invalid/") {
			k = path.Join("invalid", k)
		}
		nerr[strings.TrimSuffix(k, ".toml")] = v
	}
	r.Errors = nerr

	var (
		tests = Tests{
			Tests:   make([]Test, 0, len(r.RunTests)),
			Skipped: skipped,
		}
		limit = make(chan struct{}, r.Parallel)
		wg    sync.WaitGroup
		mu    sync.Mutex
	)
	for _, p := range r.RunTests {
		t := Test{
			Path:       p,
			Timeout:    r.Timeout,
			IntAsFloat: r.IntAsFloat,
		}
		if r.Encoder == nil && t.Encoder() {
			continue
		}
		if r.hasSkip(p) && !r.SkipMustError {
			tests.Skipped++
			mu.Lock()
			t.Skipped = true
			tests.Tests = append(tests.Tests, t)
			mu.Unlock()
			continue
		}

		limit <- struct{}{}
		wg.Add(1)
		go func(p string) {
			defer func() { <-limit; wg.Done() }()

			cmd := r.Decoder
			if t.Encoder() {
				cmd = r.Encoder
			}
			t = t.Run(cmd, r.Files)

			mu.Lock()
			if e, ok := r.Errors[p]; t.Invalid() && ok && !t.Failed() && !strings.Contains(t.Output, e) {
				t.Failure = fmt.Sprintf("%q does not contain %q", t.Output, e)
			}
			delete(r.Errors, p)

			if r.SkipMustError && r.hasSkip(p) {
				if t.Failed() {
					tests.Skipped++
					t.Skipped = true
					t.Failure = ""
				} else {
					t.Failure = "Test skipped with -skip but didn't fail"
					if t.Invalid() {
						tests.FailedInvalid++
					} else if t.Encoder() {
						tests.FailedEncoder++
					} else {
						tests.FailedValid++
					}
				}
			} else if t.Failed() {
				if t.Invalid() {
					tests.FailedInvalid++
				} else if t.Encoder() {
					tests.FailedEncoder++
				} else {
					tests.FailedValid++
				}
			} else {
				if t.Invalid() {
					tests.PassedInvalid++
				} else if t.Encoder() {
					tests.PassedEncoder++
				} else {
					tests.PassedValid++
				}
			}
			tests.Tests = append(tests.Tests, t)
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	// Sort valid first, encoder second, and invalid last.
	tr := strings.NewReplacer("encoder/", "wencoder/", "invalid/", "zinvalid/")
	sort.Slice(tests.Tests, func(i, j int) bool {
		return tr.Replace(tests.Tests[i].Path) < tr.Replace(tests.Tests[j].Path)
	})

	if len(r.Errors) > 0 {
		keys := make([]string, 0, len(r.Errors))
		for k := range r.Errors {
			keys = append(keys, k)
		}
		return tests, fmt.Errorf("errors didn't match anything: %q", keys)
	}
	return tests, nil
}

// find all TOML files in 'path' relative to the test directory.
func (r Runner) findTOML(path string, appendTo *[]string, exclude []string) error {
	// It's okay if the directory doesn't exist. Mainly to make testing a bit
	// easier.
	if _, err := fs.Stat(r.Files, path); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return fs.WalkDir(r.Files, path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".toml") {
			return nil
		}

		path = strings.TrimSuffix(path, ".toml")
		for _, e := range exclude {
			if ok, _ := filepath.Match(e, path); ok {
				return nil
			}
		}

		*appendTo = append(*appendTo, path)
		return nil
	})
}

// Expand RunTest glob patterns, or return all tests if RunTests if empty.
func (r *Runner) findTests() (int, error) {
	ls, err := r.List()
	if err != nil {
		return 0, err
	}

	var skip int
	if len(r.RunTests) == 0 {
		r.RunTests = ls
	} else {
		run := make([]string, 0, len(r.RunTests))
		for _, l := range ls {
			for _, r := range r.RunTests {
				if m, _ := filepath.Match(r, l); m {
					run = append(run, l)
					break
				}
			}
		}
		r.RunTests, skip = run, len(ls)-len(run)
	}

	// Expand invalid tests ending in ".multi.toml"
	expanded := make([]string, 0, len(r.RunTests))
	for _, path := range r.RunTests {
		if !strings.HasSuffix(path, ".multi") {
			expanded = append(expanded, path)
			continue
		}

		d, err := fs.ReadFile(r.Files, path+".toml")
		if err != nil {
			return 0, err
		}

		fmt.Println(string(d))
	}
	r.RunTests = expanded

	return skip, nil
}

func (r Runner) hasSkip(path string) bool {
	for _, s := range r.SkipTests {
		if m, _ := filepath.Match(s, path); m {
			return true
		}
	}
	return false
}

// CommandParser calls an external command.
type CommandParser struct {
	cmd []string
}

func (c CommandParser) Cmd() []string { return c.cmd }

func (c CommandParser) Run(ctx context.Context, input string) (pid int, output string, outputIsError bool, err error) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, c.cmd[0])
	cmd.Args = c.cmd
	cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader(input), stdout, stderr

	err = cmd.Run()
	if err != nil {
		eErr := &exec.ExitError{}
		if errors.As(err, &eErr) && eErr.ExitCode() == 1 {
			fmt.Fprintf(stderr, "\nExit %d\n", eErr.ProcessState.ExitCode())
			err = nil
		}
	}

	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	if stderr.Len() > 0 {
		return pid, strings.TrimSpace(stderr.String()) + "\n", true, err
	}
	return pid, strings.TrimSpace(stdout.String()) + "\n", false, err
}

func NewCommandParser(cmd []string) CommandParser {
	return CommandParser{cmd}
}

// Run this test.
func (t Test) Run(p Parser, fsys fs.FS) Test {
	if t.Invalid() {
		return t.runInvalid(p, fsys)
	}
	return t.runValid(p, fsys)
}

func (t Test) runInvalid(p Parser, fsys fs.FS) Test {
	var err error
	_, t.Input, err = t.ReadInput(fsys)
	if err != nil {
		return t.bug(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	t.PID, t.Output, t.OutputFromStderr, err = p.Run(ctx, t.Input)
	if ctx.Err() != nil {
		err = timeoutError{t.Timeout}
	}
	if err != nil {
		return t.fail(err.Error())
	}
	if !t.OutputFromStderr {
		return t.fail("Expected an error, but no error was reported.")
	}
	return t
}

func (t Test) runValid(p Parser, fsys fs.FS) Test {
	var err error
	_, t.Input, err = t.ReadInput(fsys)
	if err != nil {
		return t.bug(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	t.PID, t.Output, t.OutputFromStderr, err = p.Run(ctx, t.Input)
	if ctx.Err() != nil {
		err = timeoutError{t.Timeout}
	}
	if err != nil {
		return t.fail(err.Error())
	}
	if t.OutputFromStderr {
		return t.fail(t.Output)
	}
	if t.Output == "" {
		return t.fail("stdout is empty")
	}

	// Compare for encoder test
	if t.Encoder() {
		want, err := t.ReadWantTOML(fsys)
		if err != nil {
			return t.bug(err.Error())
		}
		var have any
		if _, err := toml.Decode(t.Output, &have); err != nil {
			return t.failf("decode TOML from encoder:\n  %s", err)
		}
		return t.CompareTOML(want, have)
	}

	// Compare for decoder test
	want, err := t.ReadWantJSON(fsys)
	if err != nil {
		return t.fail(err.Error())
	}

	var have any
	if err := json.Unmarshal([]byte(t.Output), &have); err != nil {
		return t.failf("decode JSON output from parser:\n  %s", err)
	}

	return t.CompareJSON(want, have)
}

// ReadInput reads the file sent to the encoder.
func (t Test) ReadInput(fsys fs.FS) (path, data string, err error) {
	path = t.Path + map[bool]string{true: ".json", false: ".toml"}[t.Encoder()]
	d, err := fs.ReadFile(fsys, path)
	if err != nil {
		return path, "", err
	}
	return path, string(d), nil
}

func (t Test) ReadWant(fsys fs.FS) (path, data string, err error) {
	if t.Invalid() {
		panic("testoml.Test.ReadWant: invalid tests do not have a 'correct' version")
	}

	path = t.Path + map[bool]string{true: ".toml", false: ".json"}[t.Encoder()]
	d, err := fs.ReadFile(fsys, path)
	if err != nil {
		return path, "", err
	}
	return path, string(d), nil
}

func (t *Test) ReadWantJSON(fsys fs.FS) (v any, err error) {
	var path string
	path, t.Want, err = t.ReadWant(fsys)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(t.Want), &v); err != nil {
		return nil, fmt.Errorf("decode JSON file %q:\n  %s", path, err)
	}
	return v, nil
}
func (t *Test) ReadWantTOML(fsys fs.FS) (v any, err error) {
	var path string
	path, t.Want, err = t.ReadWant(fsys)
	if err != nil {
		return nil, err
	}
	_, err = toml.Decode(t.Want, &v)
	if err != nil {
		return nil, fmt.Errorf("could not decode TOML file %q:\n  %s", path, err)
	}

	// All numbers are floats, but assume that natural numbers are encoded
	// without the .0.
	if t.IntAsFloat {
		if vv, ok := v.(map[string]any); ok {
			v = floatToInt(vv)
		}
	}

	return v, nil
}

func floatToInt(m map[string]any) map[string]any {
	newm := make(map[string]any)
	for k, v := range m {
		switch vv := v.(type) {
		case float64:
			_, frac := math.Modf(vv)
			maxSafeFloat := float64(9007199254740991)
			if !math.IsNaN(vv) && !math.IsInf(vv, 0) && vv <= maxSafeFloat && frac == 0 {
				v = int64(vv)
			}
		case map[string]any:
			v = floatToInt(vv)
		case []map[string]any:
			arr := make([]map[string]any, len(vv))
			for i, t := range vv {
				arr[i] = floatToInt(t)
			}
			v = arr
		case []any:
			arr := make([]any, len(vv))
			for i, t := range vv {
				// TODO: doesn't process nested arrays. It's okay for now as no
				// tests need it.
				switch tt := t.(type) {
				case float64:
					if !math.IsNaN(tt) && !math.IsInf(tt, 0) && tt == math.Round(tt) {
						t = int64(tt)
					}
				case map[string]any:
					t = floatToInt(tt)
				}
				arr[i] = t
			}
			v = arr
		}
		newm[k] = v
	}
	return newm
}

// Test type: "valid", "encoder", "invalid"
func (t Test) Type() testType {
	if strings.HasPrefix(t.Path, "invalid") {
		return TypeInvalid
	}
	if strings.HasPrefix(t.Path, "encoder") {
		return TypeEncoder
	}
	return TypeValid
}

func (t Test) Encoder() bool { return t.Type() == TypeEncoder }
func (t Test) Invalid() bool { return t.Type() == TypeInvalid }

func (t Test) fail(msg string) Test {
	t.Failure = msg
	return t
}

func (t Test) failf(format string, v ...any) Test {
	t.Failure = fmt.Sprintf(format, v...)
	return t
}
func (t Test) bug(format string, v ...any) Test {
	return t.failf("BUG IN TEST CASE: "+format, v...)
}

func (t Test) Failed() bool { return t.Failure != "" }
