// Package updater предоставляет логику проверки и установки обновлений wgmesh.
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	RepoOwner = "Chex4ever"
	RepoName  = "wgmesh"
	GitHubAPI = "https://api.github.com/repos/" + RepoOwner + "/" + RepoName + "/releases/latest"
)

// GHRelease — структура ответа GitHub Releases API.
type GHRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckForUpdate проверяет наличие более новой версии в GitHub Release.
func CheckForUpdate(currentVersion string) (bool, string, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", GitHubAPI, nil)
	if err != nil {
		return false, "", "", fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("User-Agent", "wgmesh-updater")

	resp, err := client.Do(req)
	if err != nil {
		return false, "", "", fmt.Errorf("ошибка соединения с GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", "", fmt.Errorf("GitHub API вернул статус: %s", resp.Status)
	}

	var rel GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return false, "", "", fmt.Errorf("ошибка разбора ответа GitHub API: %w", err)
	}

	latestVersion := strings.TrimSpace(rel.TagName)
	cleanCurrent := strings.TrimSpace(currentVersion)

	if cleanCurrent != "dev" && cleanCurrent == latestVersion {
		return false, latestVersion, "", nil
	}

	// Ищем asset c wgmesh.exe (или любой подходящий asset)
	var downloadURL string
	for _, asset := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), ".exe") || asset.Name == "wgmesh" {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" && len(rel.Assets) > 0 {
		downloadURL = rel.Assets[0].BrowserDownloadURL
	}

	if downloadURL == "" {
		return false, latestVersion, "", fmt.Errorf("в релизе %s не найден исполнимый файл для скачивания", latestVersion)
	}

	return true, latestVersion, downloadURL, nil
}

// PerformUpdate скачивает новую версию и запускает процесс обновления.
func PerformUpdate(downloadURL string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось определить путь к бинарнику: %w", err)
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("ошибка резолва симлинка: %w", err)
	}

	dir := filepath.Dir(exePath)
	tempExePath := filepath.Join(dir, "wgmesh_new.exe")

	// 1. Скачиваем файл во временное место
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("ошибка скачивания обновления: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ошибка скачивания: HTTP %s", resp.Status)
	}

	out, err := os.Create(tempExePath)
	if err != nil {
		return fmt.Errorf("ошибка создания временного файла %s: %w", tempExePath, err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tempExePath)
		return fmt.Errorf("ошибка сохранения бинарника: %w", err)
	}

	// 2. В зависимости от ОС организуем замену бинарника
	if runtime.GOOS == "windows" {
		batPath := filepath.Join(dir, "update.bat")
		batContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
del "%s"
move "%s" "%s"
start "" "%s"
del "%%~f0"
`, exePath, tempExePath, exePath, exePath)

		if err := os.WriteFile(batPath, []byte(batContent), 0755); err != nil {
			return fmt.Errorf("ошибка создания скрипта обновления: %w", err)
		}

		// Запускаем bat-скрипт асинхронно
		cmd := exec.Command("cmd.exe", "/C", batPath)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("ошибка запуска скрипта обновления: %w", err)
		}

		os.Exit(0)
	} else {
		// POSIX (Linux / macOS)
		if err := os.Chmod(tempExePath, 0755); err != nil {
			return fmt.Errorf("ошибка chmod: %w", err)
		}
		if err := os.Rename(tempExePath, exePath); err != nil {
			return fmt.Errorf("ошибка замены файла: %w", err)
		}
		// Перезапуск
		cmd := exec.Command(exePath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("ошибка перезапуска программы: %w", err)
		}
		os.Exit(0)
	}

	return nil
}
