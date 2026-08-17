package anki

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExtractMedia reads the media map and extracts files from the zip reader to target media directory.
func ExtractMedia(zipFiles []*zip.File, targetMediaDir string) (map[string]string, int, error) {
	var mediaFile *zip.File
	filesByName := make(map[string]*zip.File)

	for _, f := range zipFiles {
		filesByName[f.Name] = f
		if f.Name == "media" {
			mediaFile = f
		}
	}

	if mediaFile == nil {
		// No media file found in archive
		return nil, 0, nil
	}

	rc, err := mediaFile.Open()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open media mapping in zip: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read media mapping: %w", err)
	}

	// Anki media mapping is a JSON map: {"0": "image.png", "1": "audio.mp3"}
	mediaMap := make(map[string]string)
	if err := json.Unmarshal(data, &mediaMap); err != nil {
		// If media map is not valid json, ignore gracefully
		return nil, 0, nil
	}

	if len(mediaMap) == 0 {
		return mediaMap, 0, nil
	}

	if err := os.MkdirAll(targetMediaDir, 0755); err != nil {
		return nil, 0, fmt.Errorf("failed to create media directory %s: %w", targetMediaDir, err)
	}

	extractedCount := 0
	for zipKey, origName := range mediaMap {
		zf, exists := filesByName[zipKey]
		if !exists {
			continue
		}

		destPath := filepath.Join(targetMediaDir, origName)
		if err := extractZipFile(zf, destPath); err == nil {
			extractedCount++
		}
	}

	return mediaMap, extractedCount, nil
}

func extractZipFile(zf *zip.File, destPath string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zf.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, rc)
	return err
}
