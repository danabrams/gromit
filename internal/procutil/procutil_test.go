package procutil

import (
	"context"
	"errors"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSetProcessGroupKill(t *testing.T) {
	cmd := exec.Command("echo", "test")
	SetProcessGroupKill(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("expected Setpgid to be true")
	}
	if cmd.Cancel == nil {
		t.Fatal("expected Cancel to be set")
	}
}

func TestSetProcessGroupKillCancelNilProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	SetProcessGroupKill(cmd)

	// Cancel with nil Process should not panic
	err := cmd.Cancel()
	if err != nil {
		t.Fatalf("expected nil error for nil process, got %v", err)
	}
}

func TestSetProcessGroupKillSysProcAttrValues(t *testing.T) {
	cmd := exec.Command("echo", "test")
	SetProcessGroupKill(cmd)

	want := &syscall.SysProcAttr{Setpgid: true}
	if cmd.SysProcAttr.Setpgid != want.Setpgid {
		t.Fatalf("SysProcAttr.Setpgid = %v, want %v", cmd.SysProcAttr.Setpgid, want.Setpgid)
	}
}

func TestSubprocessEnvSetsGOMAXPROCS(t *testing.T) {
	env := SubprocessEnv()
	found := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "GOMAXPROCS=") {
			if kv != "GOMAXPROCS="+MaxGoParallelism {
				t.Fatalf("GOMAXPROCS = %q, want %q", kv, "GOMAXPROCS="+MaxGoParallelism)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("GOMAXPROCS not found in SubprocessEnv()")
	}
}

func TestSubprocessEnvRespectsExistingGOMAXPROCS(t *testing.T) {
	t.Setenv("GOMAXPROCS", "99")
	env := SubprocessEnv()
	for _, kv := range env {
		if strings.HasPrefix(kv, "GOMAXPROCS=") {
			if kv != "GOMAXPROCS=99" {
				t.Fatalf("GOMAXPROCS = %q, want %q", kv, "GOMAXPROCS=99")
			}
			return
		}
	}
	t.Fatal("GOMAXPROCS not found in SubprocessEnv()")
}

func TestSubprocessEnvRespectsRuntimeLimit(t *testing.T) {
	prev, had := os.LookupEnv("GOMAXPROCS")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GOMAXPROCS", prev)
			return
		}
		_ = os.Unsetenv("GOMAXPROCS")
	})
	if err := os.Unsetenv("GOMAXPROCS"); err != nil {
		t.Fatalf("Unsetenv: %v", err)
	}

	original := runtime.GOMAXPROCS(0)
	t.Cleanup(func() {
		runtime.GOMAXPROCS(original)
	})
	runtime.GOMAXPROCS(2)

	env := SubprocessEnv()
	for _, kv := range env {
		if strings.HasPrefix(kv, "GOMAXPROCS=") {
			t.Fatalf("expected no GOMAXPROCS override when runtime limit <= %s, found %q", MaxGoParallelism, kv)
		}
	}
}

func TestReapProcessGroupNilProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	// Should not panic when Process is nil
	ReapProcessGroup(cmd)
}

func TestReapProcessGroupAfterExit(t *testing.T) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "true")
	SetProcessGroupKill(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	// Should not panic on already-exited process group (ESRCH expected)
	ReapProcessGroup(cmd)
}

func TestReapProcessTreeNilProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	// Should not panic when Process is nil
	ReapProcessTree(cmd)
}

func TestReapProcessTreeAfterExit(t *testing.T) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "true")
	SetProcessGroupKill(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	// Should not panic on already-exited process (ESRCH expected)
	ReapProcessTree(cmd)
}

func TestReapProcessTreeKillsChildren(t *testing.T) {
	// Start a shell that spawns a backgrounded sleep, writes its PID to a
	// temp file, then exits. We use a temp file because the backgrounded
	// child inherits stdout, which would cause cmd.Output() to hang.
	pidFile := t.TempDir() + "/child.pid"
	ctx := context.Background()
	script := "sleep 300 </dev/null >/dev/null 2>&1 & echo $! > " + pidFile
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	childPidStr := strings.TrimSpace(string(data))

	// Verify the child is still alive.
	childProc, err := os.FindProcess(mustAtoi(t, childPidStr))
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	// Signal 0 checks existence without killing.
	if err := childProc.Signal(syscall.Signal(0)); err != nil {
		t.Skipf("child process already gone before reap, skipping: %v", err)
	}

	ReapProcessTree(cmd)

	// Give the kernel a moment to deliver the signal.
	time.Sleep(50 * time.Millisecond)

	// The child should now be dead.
	if err := childProc.Signal(syscall.Signal(0)); err == nil {
		t.Error("child process still alive after ReapProcessTree")
		// Clean up.
		_ = childProc.Kill()
	}
}

func TestCollectDescendantsCurrentProcess(t *testing.T) {
	// The current process has at least one thread; collectDescendants
	// should return without error (though it likely finds no children).
	pids := collectDescendants(os.Getpid())
	// Just verify it doesn't panic; pids may be empty.
	_ = pids
}

func TestCollectDescendantsInvalidPID(t *testing.T) {
	// A non-existent PID should return nil, not panic.
	pids := collectDescendants(999999999)
	if pids != nil {
		t.Fatalf("expected nil for invalid PID, got %v", pids)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("mustAtoi: non-digit in %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func TestParsePIDCount(t *testing.T) {
	got, err := parsePIDCount("42\n")
	if err != nil {
		t.Fatalf("parsePIDCount() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("parsePIDCount() = %d, want 42", got)
	}
}

func TestParsePIDLimitUnlimited(t *testing.T) {
	got, unlimited, err := parsePIDLimit("max\n")
	if err != nil {
		t.Fatalf("parsePIDLimit() error = %v", err)
	}
	if !unlimited {
		t.Fatal("parsePIDLimit() unlimited = false, want true")
	}
	if got != 0 {
		t.Fatalf("parsePIDLimit() count = %d, want 0", got)
	}
}

func TestParsePIDLimitNumeric(t *testing.T) {
	got, unlimited, err := parsePIDLimit("12000")
	if err != nil {
		t.Fatalf("parsePIDLimit() error = %v", err)
	}
	if unlimited {
		t.Fatal("parsePIDLimit() unlimited = true, want false")
	}
	if got != 12000 {
		t.Fatalf("parsePIDLimit() count = %d, want 12000", got)
	}
}

func TestWaitForProcessCapacityReturnsImmediatelyWhenNotPressured(t *testing.T) {
	original := processCreationPressuredFn
	processCreationPressuredFn = func() (bool, error) { return false, nil }
	t.Cleanup(func() {
		processCreationPressuredFn = original
	})

	if err := WaitForProcessCapacity(context.Background(), 50*time.Millisecond); err != nil {
		t.Fatalf("WaitForProcessCapacity() error = %v, want nil", err)
	}
}

func TestWaitForProcessCapacityHonorsContextCancel(t *testing.T) {
	original := processCreationPressuredFn
	processCreationPressuredFn = func() (bool, error) { return true, nil }
	t.Cleanup(func() {
		processCreationPressuredFn = original
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := WaitForProcessCapacity(ctx, 50*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitForProcessCapacity() error = %v, want context.Canceled", err)
	}
}

func TestSleepWithContextHonorsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := SleepWithContext(ctx, 50*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SleepWithContext() error = %v, want context.Canceled", err)
	}
}

func TestKillDescendantsOnCancelNilProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	// cmd.Process is nil before Start(); should not panic.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	KillDescendantsOnCancel(ctx, cmd)
}

func TestKillDescendantsOnCancelGoroutineExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "sleep", "300")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	KillDescendantsOnCancel(ctx, cmd)

	// Cancel the context; the goroutine should fire and kill the process.
	cancel()

	// Wait for the process to exit (killed by context or goroutine).
	_ = cmd.Wait()

	// Give the kernel a moment to deliver signals.
	time.Sleep(50 * time.Millisecond)

	// The process should be dead.
	if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
		t.Error("process still alive after KillDescendantsOnCancel fired")
		_ = cmd.Process.Kill()
	}
}

func TestWaitForProcessCapacityReturnsErrorWhenStillPressured(t *testing.T) {
	originalPressured := processCreationPressuredFn
	originalPressure := pidPressureFn
	processCreationPressuredFn = func() (bool, error) { return true, nil }
	pidPressureFn = func() (int, int, error) { return 90, 100, nil }
	t.Cleanup(func() {
		processCreationPressuredFn = originalPressured
		pidPressureFn = originalPressure
	})

	err := WaitForProcessCapacity(context.Background(), 1*time.Millisecond)
	if err == nil {
		t.Fatal("WaitForProcessCapacity() error = nil, want non-nil")
	}

	var capacityErr *ProcessCapacityError
	if !errors.As(err, &capacityErr) {
		t.Fatalf("WaitForProcessCapacity() error type = %T, want *ProcessCapacityError", err)
	}
	if capacityErr.Current != 90 || capacityErr.Max != 100 {
		t.Fatalf("ProcessCapacityError = (%d/%d), want (90/100)", capacityErr.Current, capacityErr.Max)
	}
}

func TestWaitForProcessCapacityUsesDefaultMaxWait(t *testing.T) {
	originalPressured := processCreationPressuredFn
	originalPressure := pidPressureFn
	originalTimeNow := timeNowFn
	originalSleep := sleepWithContextFn
	clock := &fakeClock{now: time.Unix(0, 0)}
	processCreationPressuredFn = func() (bool, error) { return true, nil }
	pidPressureFn = func() (int, int, error) { return 10, 10, nil }
	t.Cleanup(func() {
		processCreationPressuredFn = originalPressured
		pidPressureFn = originalPressure
		timeNowFn = originalTimeNow
		sleepWithContextFn = originalSleep
	})

	timeNowFn = func() time.Time { return clock.Now() }
	sleepWithContextFn = func(ctx context.Context, d time.Duration) error {
		clock.Advance(d)
		return nil
	}

	err := WaitForProcessCapacity(context.Background(), 0)
	if err == nil {
		t.Fatal("WaitForProcessCapacity() error = nil, want ProcessCapacityError")
	}
	var capErr *ProcessCapacityError
	if !errors.As(err, &capErr) {
		t.Fatalf("WaitForProcessCapacity() error type = %T, want *ProcessCapacityError", err)
	}
	if capErr.Waited != DefaultProcessCapacityMaxWait {
		t.Fatalf("WaitForProcessCapacity() waited = %v, want %v", capErr.Waited, DefaultProcessCapacityMaxWait)
	}
}

func TestWaitForProcessCapacityAnchorsDeadlineToStart(t *testing.T) {
	originalTimeNow := timeNowFn
	originalSleep := sleepWithContextFn
	originalPressured := processCreationPressuredFn
	originalPressure := pidPressureFn
	clock := &fakeClock{now: time.Unix(0, 0)}
	delta := 5 * time.Millisecond
	maxWait := 20 * time.Millisecond
	callCount := 0
	t.Cleanup(func() {
		timeNowFn = originalTimeNow
		sleepWithContextFn = originalSleep
		processCreationPressuredFn = originalPressured
		pidPressureFn = originalPressure
	})

	timeNowFn = func() time.Time {
		callCount++
		if callCount == 2 {
			clock.Advance(delta)
		}
		return clock.Now()
	}
	sleepWithContextFn = func(ctx context.Context, d time.Duration) error {
		clock.Advance(d)
		return nil
	}
	processCreationPressuredFn = func() (bool, error) { return true, nil }
	pidPressureFn = func() (int, int, error) { return 1, 1, nil }

	err := WaitForProcessCapacity(context.Background(), maxWait)
	if err == nil {
		t.Fatal("expected ProcessCapacityError")
	}
	var capErr *ProcessCapacityError
	if !errors.As(err, &capErr) {
		t.Fatalf("WaitForProcessCapacity() error type = %T, want *ProcessCapacityError", err)
	}
	if capErr.Waited != maxWait {
		t.Fatalf("Waited = %v, want %v", capErr.Waited, maxWait)
	}
}

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func TestPlatformLimitationDocComments(t *testing.T) {
	// This test verifies that ReapProcessTree, KillDescendantsOnCancel, and
	// collectDescendants have doc comments mentioning platform limitations.
	// Parse this package
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse package: %v", err)
	}

	pkg := pkgs["procutil"]
	if pkg == nil {
		t.Fatal("procutil package not found")
	}

	// Create a doc package to analyze
	docPkg := doc.New(pkg, ".", doc.AllDecls)

	// Check functions by name
	functionsToCheck := map[string]bool{
		"ReapProcessTree":         false,
		"KillDescendantsOnCancel": false,
		"collectDescendants":      false,
	}

	for _, fn := range docPkg.Funcs {
		if _, ok := functionsToCheck[fn.Name]; ok {
			// Check if the doc comment mentions platform or limitation
			docComment := fn.Doc
			hasPlatformDoc := strings.Contains(docComment, "platform") || strings.Contains(docComment, "Platform")
			if !hasPlatformDoc {
				t.Errorf("%s missing platform limitation documentation", fn.Name)
			}
			functionsToCheck[fn.Name] = true
		}
	}

	// Check that all functions were found
	for name, found := range functionsToCheck {
		if !found {
			t.Errorf("Function %s not found in documentation", name)
		}
	}
}
