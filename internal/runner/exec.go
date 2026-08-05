package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

func execCommand(cmd *exec.Cmd, command string) error {
	slog.Info(command)

	startedAt := time.Now()
	stdout, err := cmd.Output()
	if err != nil {
		elapsed := time.Since(startedAt)
		l := slog.With(slog.String("command", command))

		var stderr []byte
		if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
			l = l.With(slog.Int("exitCode", exitError.ExitCode()))
			stderr = exitError.Stderr
		}

		l.Error("command failed", slog.Duration("elapsed", elapsed))
		printCommandOutput(slog.LevelError, "stderr: ", stderr)
		return fmt.Errorf("command failed: %w", err)
	}

	slog.Info("command exited ok",
		slog.Duration("elapsed", time.Since(startedAt)))
	printCommandOutput(slog.LevelInfo, "stdout: ", stdout)
	return nil
}

func printCommandOutput(level slog.Level, prefix string, b []byte) {
	if len(b) == 0 {
		return
	}

	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		slog.LogAttrs(context.Background(), level, prefix+s.Text())
	}
}
