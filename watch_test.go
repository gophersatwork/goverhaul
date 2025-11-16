package goverhaul

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWatchMode(t *testing.T) {
	tests := []struct {
		name        string
		config      WatchConfig
		setupFS     func(afero.Fs)
		expectError bool
	}{
		{
			name: "successful creation with default config",
			config: WatchConfig{
				Path:       ".",
				ConfigPath: "config.yml",
			},
			setupFS: func(fs afero.Fs) {
				// Create a minimal valid config
				configContent := `
rules:
  - path: internal/domain
    prohibited:
      - name: internal/infrastructure
        cause: domain should not depend on infrastructure
`
				afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
				afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)
			},
			expectError: false,
		},
		{
			name: "missing config file",
			config: WatchConfig{
				Path:       ".",
				ConfigPath: "nonexistent.yml",
			},
			setupFS:     func(fs afero.Fs) {},
			expectError: true,
		},
		{
			name: "custom debounce time",
			config: WatchConfig{
				Path:         ".",
				ConfigPath:   "config.yml",
				DebounceTime: 200 * time.Millisecond,
			},
			setupFS: func(fs afero.Fs) {
				configContent := `
rules:
  - path: internal/domain
    prohibited:
      - name: internal/infrastructure
`
				afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
				afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory filesystem
			fs := afero.NewMemMapFs()
			tt.setupFS(fs)

			// Set up logger
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelError, // Reduce noise in tests
			}))

			tt.config.FS = fs
			tt.config.Logger = logger

			watchMode, err := NewWatchMode(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, watchMode)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, watchMode)
				if watchMode != nil {
					defer watchMode.Stop()

					// Verify debounce time
					if tt.config.DebounceTime != 0 {
						assert.Equal(t, tt.config.DebounceTime, watchMode.debounceTime)
					} else {
						assert.Equal(t, 100*time.Millisecond, watchMode.debounceTime)
					}
				}
			}
		})
	}
}

func TestWatchMode_AddGoFilesToWatcher(t *testing.T) {
	t.Skip("Skipping watcher tests - fsnotify doesn't work with in-memory filesystem")

	// Note: fsnotify requires a real filesystem to watch files.
	// These tests would need to use a temporary directory on the real filesystem
	// or be refactored to test the logic without actually watching files.
	// The watch functionality is tested through integration tests with real filesystem.
}

func TestWatchMode_ShouldProcessEvent(t *testing.T) {
	fs := afero.NewMemMapFs()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create minimal setup
	configContent := `rules: []`
	afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
	afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)

	watchMode, err := NewWatchMode(WatchConfig{
		Path:       ".",
		ConfigPath: "config.yml",
		FS:         fs,
		Logger:     logger,
	})
	require.NoError(t, err)
	defer watchMode.Stop()

	tests := []struct {
		name     string
		event    fsnotify.Event
		expected bool
	}{
		{
			name: "write event",
			event: fsnotify.Event{
				Name: "test.go",
				Op:   fsnotify.Write,
			},
			expected: true,
		},
		{
			name: "create event",
			event: fsnotify.Event{
				Name: "test.go",
				Op:   fsnotify.Create,
			},
			expected: true,
		},
		{
			name: "rename event",
			event: fsnotify.Event{
				Name: "test.go",
				Op:   fsnotify.Rename,
			},
			expected: true,
		},
		{
			name: "remove event",
			event: fsnotify.Event{
				Name: "test.go",
				Op:   fsnotify.Remove,
			},
			expected: false,
		},
		{
			name: "chmod event",
			event: fsnotify.Event{
				Name: "test.go",
				Op:   fsnotify.Chmod,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := watchMode.shouldProcessEvent(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWatchMode_IsConfigFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create minimal setup
	configContent := `rules: []`
	afero.WriteFile(fs, "/project/config.yml", []byte(configContent), 0644)
	afero.WriteFile(fs, "/project/go.mod", []byte("module test\n"), 0644)

	tests := []struct {
		name       string
		configPath string
		eventPath  string
		expected   bool
	}{
		{
			name:       "exact match",
			configPath: "/project/config.yml",
			eventPath:  "/project/config.yml",
			expected:   true,
		},
		{
			name:       "different file",
			configPath: "/project/config.yml",
			eventPath:  "/project/main.go",
			expected:   false,
		},
		{
			name:       "empty config path",
			configPath: "",
			eventPath:  "/project/config.yml",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watchMode, err := NewWatchMode(WatchConfig{
				Path:       "/project",
				ConfigPath: tt.configPath,
				FS:         fs,
				Logger:     logger,
			})
			require.NoError(t, err)
			defer watchMode.Stop()

			result := watchMode.isConfigFile(tt.eventPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWatchMode_AnalyzeFiles(t *testing.T) {
	tests := []struct {
		name              string
		setupFS           func(afero.Fs)
		files             []string
		expectedViolations int
	}{
		{
			name: "single file with violation",
			setupFS: func(fs afero.Fs) {
				configContent := `
rules:
  - path: internal/domain
    prohibited:
      - name: internal/infrastructure
        cause: domain should not depend on infrastructure
`
				afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
				afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)

				// File with violation
				fileContent := `package domain

import "test/internal/infrastructure"

type User struct {}
`
				afero.WriteFile(fs, "internal/domain/user.go", []byte(fileContent), 0644)
			},
			files:              []string{"internal/domain/user.go"},
			expectedViolations: 1,
		},
		{
			name: "file without violations",
			setupFS: func(fs afero.Fs) {
				configContent := `
rules:
  - path: internal/domain
    prohibited:
      - name: internal/infrastructure
`
				afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
				afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)

				// File without violation
				fileContent := `package domain

type User struct {}
`
				afero.WriteFile(fs, "internal/domain/user.go", []byte(fileContent), 0644)
			},
			files:              []string{"internal/domain/user.go"},
			expectedViolations: 0,
		},
		{
			name: "nonexistent file",
			setupFS: func(fs afero.Fs) {
				configContent := `rules: []`
				afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
				afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)
			},
			files:              []string{"internal/domain/deleted.go"},
			expectedViolations: 0,
		},
		{
			name: "multiple files",
			setupFS: func(fs afero.Fs) {
				configContent := `
rules:
  - path: internal/domain
    prohibited:
      - name: internal/infrastructure
`
				afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
				afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)

				// File 1 with violation
				file1 := `package domain

import "test/internal/infrastructure"
`
				afero.WriteFile(fs, "internal/domain/user.go", []byte(file1), 0644)

				// File 2 without violation
				file2 := `package domain

type Order struct {}
`
				afero.WriteFile(fs, "internal/domain/order.go", []byte(file2), 0644)
			},
			files:              []string{"internal/domain/user.go", "internal/domain/order.go"},
			expectedViolations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			tt.setupFS(fs)

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelError,
			}))

			watchMode, err := NewWatchMode(WatchConfig{
				Path:       ".",
				ConfigPath: "config.yml",
				FS:         fs,
				Logger:     logger,
			})
			require.NoError(t, err)
			defer watchMode.Stop()

			violations := watchMode.analyzeFiles(tt.files)
			assert.Equal(t, tt.expectedViolations, len(violations.Violations))
		})
	}
}

func TestWatchMode_Stats(t *testing.T) {
	fs := afero.NewMemMapFs()

	configContent := `rules: []`
	afero.WriteFile(fs, "config.yml", []byte(configContent), 0644)
	afero.WriteFile(fs, "go.mod", []byte("module test\n"), 0644)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	watchMode, err := NewWatchMode(WatchConfig{
		Path:       ".",
		ConfigPath: "config.yml",
		FS:         fs,
		Logger:     logger,
	})
	require.NoError(t, err)
	defer watchMode.Stop()

	// Initially zero
	stats := watchMode.GetStats()
	assert.Equal(t, 0, stats.totalAnalyses)
	assert.Equal(t, 0, stats.violationsFound)

	// Update stats
	watchMode.updateStats(5)

	stats = watchMode.GetStats()
	assert.Equal(t, 1, stats.totalAnalyses)
	assert.Equal(t, 5, stats.violationsFound)

	// Update again
	watchMode.updateStats(3)

	stats = watchMode.GetStats()
	assert.Equal(t, 2, stats.totalAnalyses)
	assert.Equal(t, 8, stats.violationsFound)
}

func TestWatchMode_GracefulShutdown(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	testDir := "/test"
	fs.MkdirAll(testDir, 0755)

	configContent := `rules: []`
	afero.WriteFile(fs, filepath.Join(testDir, "config.yml"), []byte(configContent), 0644)
	afero.WriteFile(fs, filepath.Join(testDir, "go.mod"), []byte("module test\n"), 0644)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	watchMode, err := NewWatchMode(WatchConfig{
		Path:       testDir,
		ConfigPath: filepath.Join(testDir, "config.yml"),
		FS:         fs,
		Logger:     logger,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- watchMode.Start(ctx, testDir)
	}()

	// Wait for timeout
	select {
	case err := <-errChan:
		// Should complete without error
		assert.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Watch mode did not shut down gracefully")
	}
}
