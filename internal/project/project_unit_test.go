package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ScaleCommerce-DEV/scdev/internal/config"
	"github.com/ScaleCommerce-DEV/scdev/internal/runtime"
)

// setupTestEnv creates a temporary SCDEV_HOME and HOME directory with a
// global-config.yaml that has Mutagen disabled. Returns a cleanup function.
func setupTestEnv(t *testing.T) func() {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .scdev dir inside tmp (for state.DefaultManager which uses HOME)
	scdevDir := filepath.Join(tmpDir, ".scdev")
	if err := os.MkdirAll(scdevDir, 0755); err != nil {
		t.Fatalf("failed to create .scdev dir: %v", err)
	}

	// Write global-config.yaml with Mutagen disabled
	globalConfig := []byte("version: 1\ndomain: scalecommerce.site\nruntime: docker\nmutagen:\n  enabled: \"false\"\n")
	if err := os.WriteFile(filepath.Join(scdevDir, "global-config.yaml"), globalConfig, 0644); err != nil {
		t.Fatalf("failed to write global-config.yaml: %v", err)
	}

	// Save and override env vars
	oldHome := os.Getenv("HOME")
	oldScdevHome := os.Getenv("SCDEV_HOME")

	os.Setenv("HOME", tmpDir)
	os.Setenv("SCDEV_HOME", scdevDir)

	return func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("SCDEV_HOME", oldScdevHome)
	}
}

// newTestProject creates a Project with a MockRuntime and basic config for testing.
func newTestProject(mock *runtime.MockRuntime) *Project {
	return &Project{
		Dir: "/tmp/test",
		Config: &config.ProjectConfig{
			Name: "testproject",
			Services: map[string]config.ServiceConfig{
				"app": {Image: "alpine:latest", Command: "sleep infinity"},
			},
			Volumes: map[string]config.VolumeConfig{
				"data": {},
			},
			Shared: config.ProjectSharedConfig{
				Router:        false,
				Mail:          false,
				DBUI:          false,
				RedisInsights: false,
			},
		},
		Runtime: mock,
	}
}

// =============================================================================
// parseVolumeMount tests
// =============================================================================

func TestParseVolumeMount_NamedVolume(t *testing.T) {
	source, target, isNamed := parseVolumeMount("db_data:/var/lib/data")
	if source != "db_data" {
		t.Errorf("source = %q, want %q", source, "db_data")
	}
	if target != "/var/lib/data" {
		t.Errorf("target = %q, want %q", target, "/var/lib/data")
	}
	if !isNamed {
		t.Error("isNamedVolume = false, want true")
	}
}

func TestParseVolumeMount_AbsoluteBindMount(t *testing.T) {
	source, target, isNamed := parseVolumeMount("/host/path:/container/path")
	if source != "/host/path" {
		t.Errorf("source = %q, want %q", source, "/host/path")
	}
	if target != "/container/path" {
		t.Errorf("target = %q, want %q", target, "/container/path")
	}
	if isNamed {
		t.Error("isNamedVolume = true, want false")
	}
}

func TestParseVolumeMount_RelativeBindMount(t *testing.T) {
	source, target, isNamed := parseVolumeMount("./src:/app")
	if source != "./src" {
		t.Errorf("source = %q, want %q", source, "./src")
	}
	if target != "/app" {
		t.Errorf("target = %q, want %q", target, "/app")
	}
	if isNamed {
		t.Error("isNamedVolume = true, want false")
	}
}

func TestParseVolumeMount_DotPath(t *testing.T) {
	source, target, isNamed := parseVolumeMount(".:/var/www/html")
	if source != "." {
		t.Errorf("source = %q, want %q", source, ".")
	}
	if target != "/var/www/html" {
		t.Errorf("target = %q, want %q", target, "/var/www/html")
	}
	if isNamed {
		t.Error("isNamedVolume = true, want false")
	}
}

func TestParseVolumeMount_NoColon(t *testing.T) {
	source, target, isNamed := parseVolumeMount("just-a-name")
	if source != "just-a-name" {
		t.Errorf("source = %q, want %q", source, "just-a-name")
	}
	if target != "just-a-name" {
		t.Errorf("target = %q, want %q", target, "just-a-name")
	}
	if isNamed {
		t.Error("isNamedVolume = true, want false")
	}
}

func TestParseVolumeMount_MultipleColons(t *testing.T) {
	// SplitN with n=2 splits on the first colon only
	// "C:\path:/container" -> first colon is after "C", so source="C", target="\path:/container"
	source, target, isNamed := parseVolumeMount("C:\\path:/container")
	if source != "C" {
		t.Errorf("source = %q, want %q", source, "C")
	}
	if target != "\\path:/container" {
		t.Errorf("target = %q, want %q", target, "\\path:/container")
	}
	// "C" doesn't start with / or . so it's treated as a named volume
	if !isNamed {
		t.Error("isNamedVolume = false, want true ('C' doesn't start with / or .)")
	}
}

// =============================================================================
// transformVolumesForMutagen tests
// =============================================================================

func TestTransformVolumesForMutagen_NamedVolumePassesThrough(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	mutagenMounts := map[string]MutagenSyncMount{}
	volumes := []string{"db_data:/var/lib/data"}

	result := p.transformVolumesForMutagen("app", volumes, mutagenMounts)

	if len(result) != 1 {
		t.Fatalf("got %d volumes, want 1", len(result))
	}
	// Named volume should be prefixed with project name
	expected := "db_data.testproject.scdev"
	if result[0].Source != expected {
		t.Errorf("source = %q, want %q", result[0].Source, expected)
	}
	if result[0].Target != "/var/lib/data" {
		t.Errorf("target = %q, want %q", result[0].Target, "/var/lib/data")
	}
}

func TestTransformVolumesForMutagen_BindMountWithMutagenMatch(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	mutagenMounts := map[string]MutagenSyncMount{
		"app": {
			ServiceName:   "app",
			HostPath:      "/tmp/test",
			ContainerPath: "/var/www/html",
			VolumeName:    "sync.app.testproject.scdev",
			SessionName:   "scdev-testproject-app",
		},
	}
	volumes := []string{".:/var/www/html"}

	result := p.transformVolumesForMutagen("app", volumes, mutagenMounts)

	if len(result) != 1 {
		t.Fatalf("got %d volumes, want 1", len(result))
	}
	if result[0].Source != "sync.app.testproject.scdev" {
		t.Errorf("source = %q, want %q", result[0].Source, "sync.app.testproject.scdev")
	}
	if result[0].Target != "/var/www/html" {
		t.Errorf("target = %q, want %q", result[0].Target, "/var/www/html")
	}
}

func TestTransformVolumesForMutagen_BindMountWithoutMutagenMatch(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	mutagenMounts := map[string]MutagenSyncMount{}
	volumes := []string{"./config:/etc/app"}

	result := p.transformVolumesForMutagen("app", volumes, mutagenMounts)

	if len(result) != 1 {
		t.Fatalf("got %d volumes, want 1", len(result))
	}
	// Should stay as bind mount
	if result[0].Source != "./config" {
		t.Errorf("source = %q, want %q", result[0].Source, "./config")
	}
	if result[0].Target != "/etc/app" {
		t.Errorf("target = %q, want %q", result[0].Target, "/etc/app")
	}
}

func TestTransformVolumesForMutagen_MixedVolumes(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	mutagenMounts := map[string]MutagenSyncMount{
		"app": {
			ServiceName:   "app",
			HostPath:      "/tmp/test",
			ContainerPath: "/app",
			VolumeName:    "sync.app.testproject.scdev",
			SessionName:   "scdev-testproject-app",
		},
	}
	volumes := []string{
		"db_data:/var/lib/data",  // named volume
		".:/app",                 // bind mount with Mutagen match
		"./logs:/var/log/app",    // bind mount without Mutagen match
	}

	result := p.transformVolumesForMutagen("app", volumes, mutagenMounts)

	if len(result) != 3 {
		t.Fatalf("got %d volumes, want 3", len(result))
	}

	// Named volume - prefixed
	if result[0].Source != "db_data.testproject.scdev" {
		t.Errorf("vol[0] source = %q, want %q", result[0].Source, "db_data.testproject.scdev")
	}

	// Bind mount replaced with Mutagen volume
	if result[1].Source != "sync.app.testproject.scdev" {
		t.Errorf("vol[1] source = %q, want %q", result[1].Source, "sync.app.testproject.scdev")
	}

	// Bind mount without match stays as-is
	if result[2].Source != "./logs" {
		t.Errorf("vol[2] source = %q, want %q", result[2].Source, "./logs")
	}
}

// =============================================================================
// Naming convention tests
// =============================================================================

func TestContainerName(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	got := p.ContainerName("app")
	want := "app.testproject.scdev"
	if got != want {
		t.Errorf("ContainerName = %q, want %q", got, want)
	}

	got = p.ContainerName("db")
	want = "db.testproject.scdev"
	if got != want {
		t.Errorf("ContainerName = %q, want %q", got, want)
	}
}

func TestNetworkName(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	got := p.NetworkName()
	want := "testproject.scdev"
	if got != want {
		t.Errorf("NetworkName = %q, want %q", got, want)
	}
}

func TestVolumeName(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	got := p.VolumeName("data")
	want := "data.testproject.scdev"
	if got != want {
		t.Errorf("VolumeName = %q, want %q", got, want)
	}

	got = p.VolumeName("db_data")
	want = "db_data.testproject.scdev"
	if got != want {
		t.Errorf("VolumeName = %q, want %q", got, want)
	}
}

func TestMutagenSessionName(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	got := p.MutagenSessionName("app")
	want := "scdev-testproject-app"
	if got != want {
		t.Errorf("MutagenSessionName = %q, want %q", got, want)
	}
}

func TestMutagenVolumeName(t *testing.T) {
	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	got := p.MutagenVolumeName("app")
	want := "sync.app.testproject.scdev"
	if got != want {
		t.Errorf("MutagenVolumeName = %q, want %q", got, want)
	}
}

// =============================================================================
// Lifecycle tests: Start
// =============================================================================

func TestStart_CreatesNetworkAndVolumesAndContainers(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Network should be created
	if !mock.NetworksExist["testproject.scdev"] {
		t.Error("network testproject.scdev was not created")
	}

	// Volume should be created
	if !mock.VolumesExist["data.testproject.scdev"] {
		t.Error("volume data.testproject.scdev was not created")
	}

	// Container should exist and be running
	containerName := "app.testproject.scdev"
	if !mock.ContainersExist[containerName] {
		t.Error("container app.testproject.scdev was not created")
	}
	if !mock.ContainersRunning[containerName] {
		t.Error("container app.testproject.scdev is not running")
	}

	// Verify CreateNetwork was called
	if mock.CallCount("CreateNetwork") != 1 {
		t.Errorf("CreateNetwork called %d times, want 1", mock.CallCount("CreateNetwork"))
	}

	// Verify CreateVolume was called
	if mock.CallCount("CreateVolume") != 1 {
		t.Errorf("CreateVolume called %d times, want 1", mock.CallCount("CreateVolume"))
	}
}

func TestStart_SkipsNetworkCreationIfExists(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.NetworksExist["testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Network should not have been created again
	if mock.CallCount("CreateNetwork") != 0 {
		t.Errorf("CreateNetwork called %d times, want 0 (network already existed)", mock.CallCount("CreateNetwork"))
	}
}

func TestStart_SkipsVolumeCreationIfExists(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.VolumesExist["data.testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if mock.CallCount("CreateVolume") != 0 {
		t.Errorf("CreateVolume called %d times, want 0 (volume already existed)", mock.CallCount("CreateVolume"))
	}
}

func TestStart_SkipsAlreadyRunningContainer(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Should not create or start a new container
	if mock.CallCount("CreateContainer") != 0 {
		t.Errorf("CreateContainer called %d times, want 0", mock.CallCount("CreateContainer"))
	}
	// StartContainer should not be called since it's already running
	if mock.CallCount("StartContainer") != 0 {
		t.Errorf("StartContainer called %d times, want 0", mock.CallCount("StartContainer"))
	}
}

func TestStart_StartsStoppedContainer(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = false

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Should start the existing container, not create a new one
	if mock.CallCount("CreateContainer") != 0 {
		t.Errorf("CreateContainer called %d times, want 0", mock.CallCount("CreateContainer"))
	}
	if mock.CallCount("StartContainer") != 1 {
		t.Errorf("StartContainer called %d times, want 1", mock.CallCount("StartContainer"))
	}
}

// =============================================================================
// Lifecycle tests: Stop
// =============================================================================

func TestStop_StopsRunningContainers(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if mock.CallCount("StopContainer") != 1 {
		t.Errorf("StopContainer called %d times, want 1", mock.CallCount("StopContainer"))
	}
	if !mock.CalledWith("StopContainer", "app.testproject.scdev") {
		t.Error("StopContainer was not called with app.testproject.scdev")
	}
}

func TestStop_SkipsNonExistentContainers(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	// Container does not exist
	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if mock.CallCount("StopContainer") != 0 {
		t.Errorf("StopContainer called %d times, want 0", mock.CallCount("StopContainer"))
	}
}

func TestStop_SkipsAlreadyStoppedContainers(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = false

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if mock.CallCount("StopContainer") != 0 {
		t.Errorf("StopContainer called %d times, want 0 (container already stopped)", mock.CallCount("StopContainer"))
	}
}

// =============================================================================
// Lifecycle tests: Down
// =============================================================================

func TestDown_StopsAndRemovesContainersAndNetwork(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = true
	mock.NetworksExist["testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Down(ctx, false); err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	// Container should be stopped
	if mock.CallCount("StopContainer") != 1 {
		t.Errorf("StopContainer called %d times, want 1", mock.CallCount("StopContainer"))
	}

	// Container should be removed
	if mock.CallCount("RemoveContainer") != 1 {
		t.Errorf("RemoveContainer called %d times, want 1", mock.CallCount("RemoveContainer"))
	}

	// Network should be removed
	if mock.CallCount("RemoveNetwork") != 1 {
		t.Errorf("RemoveNetwork called %d times, want 1", mock.CallCount("RemoveNetwork"))
	}
}

func TestDown_SkipsStopForNonRunningContainers(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = false
	mock.NetworksExist["testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Down(ctx, false); err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	// Should not stop (already stopped), but should remove
	if mock.CallCount("StopContainer") != 0 {
		t.Errorf("StopContainer called %d times, want 0", mock.CallCount("StopContainer"))
	}
	if mock.CallCount("RemoveContainer") != 1 {
		t.Errorf("RemoveContainer called %d times, want 1", mock.CallCount("RemoveContainer"))
	}
}

func TestDown_SkipsNonExistentContainers(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.NetworksExist["testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Down(ctx, false); err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	if mock.CallCount("StopContainer") != 0 {
		t.Errorf("StopContainer called %d times, want 0", mock.CallCount("StopContainer"))
	}
	if mock.CallCount("RemoveContainer") != 0 {
		t.Errorf("RemoveContainer called %d times, want 0", mock.CallCount("RemoveContainer"))
	}
	// Network should still be removed
	if mock.CallCount("RemoveNetwork") != 1 {
		t.Errorf("RemoveNetwork called %d times, want 1", mock.CallCount("RemoveNetwork"))
	}
}

func TestDown_WithRemoveVolumes(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = true
	mock.NetworksExist["testproject.scdev"] = true
	mock.VolumesExist["data.testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Down(ctx, true); err != nil {
		t.Fatalf("Down(removeVolumes=true) error: %v", err)
	}

	// Volume should be removed
	if mock.CallCount("RemoveVolume") != 1 {
		t.Errorf("RemoveVolume called %d times, want 1", mock.CallCount("RemoveVolume"))
	}
	if !mock.CalledWith("RemoveVolume", "data.testproject.scdev") {
		t.Error("RemoveVolume was not called with data.testproject.scdev")
	}
}

func TestDown_WithoutRemoveVolumes(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.ContainersExist["app.testproject.scdev"] = true
	mock.ContainersRunning["app.testproject.scdev"] = true
	mock.NetworksExist["testproject.scdev"] = true
	mock.VolumesExist["data.testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Down(ctx, false); err != nil {
		t.Fatalf("Down(removeVolumes=false) error: %v", err)
	}

	// Volume should NOT be removed
	if mock.CallCount("RemoveVolume") != 0 {
		t.Errorf("RemoveVolume called %d times, want 0 (removeVolumes=false)", mock.CallCount("RemoveVolume"))
	}

	// Volume should still exist in mock state
	if !mock.VolumesExist["data.testproject.scdev"] {
		t.Error("volume data.testproject.scdev should still exist")
	}
}

func TestDown_RemovesNetworkEvenIfNoContainers(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	mock.NetworksExist["testproject.scdev"] = true

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Down(ctx, false); err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	if !mock.CalledWith("RemoveNetwork", "testproject.scdev") {
		t.Error("RemoveNetwork was not called with testproject.scdev")
	}
	if mock.NetworksExist["testproject.scdev"] {
		t.Error("network testproject.scdev should have been removed")
	}
}

func TestDown_SkipsNetworkRemovalIfNotExists(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	mock := runtime.NewMockRuntime()
	// No network exists

	p := newTestProject(mock)

	ctx := context.Background()
	if err := p.Down(ctx, false); err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	if mock.CallCount("RemoveNetwork") != 0 {
		t.Errorf("RemoveNetwork called %d times, want 0 (network doesn't exist)", mock.CallCount("RemoveNetwork"))
	}
}
