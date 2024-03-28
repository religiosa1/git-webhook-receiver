package action_runner

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type outputEntryInfo struct {
	Name    string
	ModTime time.Time
}

func removeOldOutputFiles(logger *slog.Logger, actionsOutputDir string, maxOutputFiles int) {
	if maxOutputFiles <= 0 {
		return
	}
	logger.Debug("Clearing up old output files")
	files, err := os.ReadDir(actionsOutputDir)
	if err != nil {
		logger.Error("Error reading output files for clean up", slog.Any("error", err))
	}

	outputEntries := make([]outputEntryInfo, 0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".stdout") && !strings.HasSuffix(file.Name(), ".stderr") {
			continue
		}
		info, err := file.Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logger.Warn(
					"Unable to get outputn file's stats while performing a clean up",
					slog.String("filename", file.Name()),
					slog.Any("error", err),
				)
			}
			continue
		}
		outputEntries = append(outputEntries, outputEntryInfo{file.Name(), info.ModTime()})
	}

	if len(outputEntries) <= maxOutputFiles {
		logger.Debug(
			"nothing to remove here",
			slog.Int("maxOutputFiles", maxOutputFiles),
			slog.Int("outputEntriesLength", len(outputEntries)),
		)
		return
	}

	sort.Slice(outputEntries, func(i, j int) bool {
		return outputEntries[i].ModTime.After(outputEntries[j].ModTime)
	})

	logger.Debug(
		"removing oldest files",
		slog.Int("length", len(outputEntries)),
		slog.Any("entriesToDelete", outputEntries[maxOutputFiles:]),
	)

	for _, file := range outputEntries[maxOutputFiles:] {
		err := os.Remove(filepath.Join(actionsOutputDir, file.Name))
		if err != nil {
			logger.Warn(
				"Error during removal of an output file",
				slog.String("filename", file.Name),
				slog.Any("error", err),
			)
		}
	}
}
