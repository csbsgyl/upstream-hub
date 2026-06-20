package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	updateContainerName = "upstreamhub-updater"
	defaultUpdateImage  = "upstream-hub:local"
	defaultDockerSocket = "/var/run/docker.sock"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type updateCapability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	HostDir   string `json:"host_dir,omitempty"`
	Image     string `json:"image,omitempty"`
}

type opsUpdateResponse struct {
	OK            bool      `json:"ok"`
	Started       bool      `json:"started"`
	Message       string    `json:"message"`
	ContainerName string    `json:"container_name"`
	LogFile       string    `json:"log_file"`
	StartedAt     time.Time `json:"started_at"`
}

type opsUpdateStatusResponse struct {
	OK         bool     `json:"ok"`
	Status     string   `json:"status"`
	Running    bool     `json:"running"`
	Completed  bool     `json:"completed"`
	Failed     bool     `json:"failed"`
	Phase      string   `json:"phase"`
	PhaseLabel string   `json:"phase_label"`
	Progress   int      `json:"progress"`
	Message    string   `json:"message"`
	LogFile    string   `json:"log_file,omitempty"`
	Lines      []string `json:"lines,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

func startSystemUpdate(c *gin.Context, d *Deps) {
	capability := detectUpdateCapability(d)
	if !capability.Available {
		audit(c, d, "ops.update", "ops", 0, "started system update", gin.H{
			"ok":     false,
			"reason": capability.Reason,
		})
		fail(c, http.StatusBadRequest, errors.New(capability.Reason))
		return
	}

	startedAt := time.Now()
	logFile := fmt.Sprintf("backups/update-%s.log", startedAt.Format("20060102-150405"))
	args := dockerUpdateArgs(capability.HostDir, dockerSocketPath(), capability.Image, logFile)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = os.Environ()
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		msg := sanitizeCommandOutput(output.String())
		if msg == "" {
			msg = err.Error()
		}
		audit(c, d, "ops.update", "ops", 0, "started system update", gin.H{
			"ok":     false,
			"error":  msg,
			"log":    logFile,
			"image":  capability.Image,
			"host":   capability.HostDir,
			"socket": dockerSocketPath(),
		})
		fail(c, http.StatusInternalServerError, fmt.Errorf("启动更新失败：%s", msg))
		return
	}

	containerID := strings.TrimSpace(output.String())
	audit(c, d, "ops.update", "ops", 0, "started system update", gin.H{
		"ok":           true,
		"container":    updateContainerName,
		"container_id": containerID,
		"log":          logFile,
		"image":        capability.Image,
		"host":         capability.HostDir,
	})
	c.JSON(http.StatusAccepted, gin.H{"data": opsUpdateResponse{
		OK:            true,
		Started:       true,
		Message:       "更新任务已启动，系统会在后台拉取代码、备份数据库并重建服务",
		ContainerName: updateContainerName,
		LogFile:       logFile,
		StartedAt:     startedAt,
	}})
}

func systemUpdateStatus(c *gin.Context, _ *Deps) {
	status, err := buildUpdateStatus(c.Query("log_file"))
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

func buildUpdateStatus(rawLogFile string) (opsUpdateStatusResponse, error) {
	logPath, logFile, ok, err := resolveUpdateLogPath(rawLogFile)
	if err != nil {
		return opsUpdateStatusResponse{}, err
	}
	if !ok {
		return opsUpdateStatusResponse{
			OK:         true,
			Status:     "idle",
			Phase:      "idle",
			PhaseLabel: "未开始",
			Progress:   0,
			Message:    "当前没有更新任务",
		}, nil
	}

	lines, updatedAt, err := readUpdateLog(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return opsUpdateStatusResponse{
				OK:         true,
				Status:     "starting",
				Running:    true,
				Phase:      "start",
				PhaseLabel: "启动任务",
				Progress:   6,
				Message:    "更新容器已提交，正在等待日志写入",
				LogFile:    logFile,
			}, nil
		}
		return opsUpdateStatusResponse{}, err
	}

	status := parseUpdateStatus(lines)
	status.OK = true
	status.LogFile = logFile
	status.Lines = tailLines(lines, 80)
	status.UpdatedAt = updatedAt.Format(time.RFC3339)
	if status.Running && time.Since(updatedAt) > 15*time.Minute && !updateContainerRunning() {
		status.Status = "unknown"
		status.Running = false
		status.Message = "未检测到完成标记，更新容器也不在运行，请查看日志确认结果"
	}
	return status, nil
}

func resolveUpdateLogPath(raw string) (path string, logFile string, ok bool, err error) {
	base := filepath.Base(strings.TrimSpace(raw))
	if base == "." || base == string(filepath.Separator) || base == "" {
		latest, found := latestUpdateLog()
		if !found {
			return "", "", false, nil
		}
		base = filepath.Base(latest)
	}
	if !strings.HasPrefix(base, "update-") || !strings.HasSuffix(base, ".log") {
		return "", "", false, fmt.Errorf("invalid update log file")
	}
	return filepath.Join(backupDir(), base), filepath.ToSlash(filepath.Join("backups", base)), true, nil
}

func latestUpdateLog() (string, bool) {
	matches, err := filepath.Glob(filepath.Join(backupDir(), "update-*.log"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	sort.Slice(matches, func(i, j int) bool {
		ai, errA := os.Stat(matches[i])
		bi, errB := os.Stat(matches[j])
		if errA != nil || errB != nil {
			return matches[i] > matches[j]
		}
		return ai.ModTime().After(bi.ModTime())
	})
	return matches[0], true
}

func readUpdateLog(path string) ([]string, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(b) > 512*1024 {
		b = b[len(b)-512*1024:]
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(ansiEscapePattern.ReplaceAllString(line, ""))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, info.ModTime(), nil
}

func parseUpdateStatus(lines []string) opsUpdateStatusResponse {
	phase := "start"
	message := "更新任务正在执行"
	failed := false
	completed := false
	for _, line := range lines {
		if marker := parseUpdateStageMarker(line); marker.phase != "" {
			phase = marker.phase
			message = updatePhaseMessage(marker.phase)
		}
		if strings.Contains(line, "[FAIL]") || strings.Contains(line, "UPSTREAMHUB_STAGE] failed|") {
			failed = true
		}
		if strings.Contains(line, "UPSTREAMHUB_STAGE] done|") || strings.Contains(line, "[ OK ] updater finished") {
			completed = true
		}
	}
	label, progress := updatePhaseMeta(phase)
	status := "running"
	running := true
	if completed {
		phase = "done"
		label, progress = updatePhaseMeta(phase)
		status = "completed"
		running = false
		message = "更新完成，服务会自动恢复到最新版本"
	}
	if failed {
		phase = "failed"
		label, progress = updatePhaseMeta(phase)
		status = "failed"
		running = false
		message = "更新失败，请查看日志尾部定位原因"
	}
	return opsUpdateStatusResponse{
		Status:     status,
		Running:    running,
		Completed:  completed,
		Failed:     failed,
		Phase:      phase,
		PhaseLabel: label,
		Progress:   progress,
		Message:    message,
	}
}

type updateStageMarker struct {
	phase   string
	message string
}

func parseUpdateStageMarker(line string) updateStageMarker {
	idx := strings.Index(line, "[UPSTREAMHUB_STAGE]")
	if idx < 0 {
		return updateStageMarker{}
	}
	payload := strings.TrimSpace(line[idx+len("[UPSTREAMHUB_STAGE]"):])
	phase, message, _ := strings.Cut(payload, "|")
	return updateStageMarker{phase: strings.TrimSpace(phase), message: strings.TrimSpace(message)}
}

func updatePhaseMessage(phase string) string {
	switch phase {
	case "check":
		return "正在检查 Docker 和 Compose 环境"
	case "env":
		return "正在准备 .env 和更新配置"
	case "pull":
		return "正在拉取仓库最新代码"
	case "backup":
		return "正在检查并执行部署前备份"
	case "build":
		return "正在构建镜像并重启服务"
	case "health":
		return "正在等待服务健康检查"
	case "done":
		return "更新完成，服务会自动恢复到最新版本"
	case "failed":
		return "更新失败，请查看日志尾部定位原因"
	default:
		return "更新任务正在执行"
	}
}

func updatePhaseMeta(phase string) (string, int) {
	switch phase {
	case "check":
		return "检查环境", 14
	case "env":
		return "准备配置", 24
	case "pull":
		return "拉取代码", 40
	case "backup":
		return "备份数据", 56
	case "build":
		return "构建并重启", 76
	case "health":
		return "健康检查", 90
	case "done":
		return "更新完成", 100
	case "failed":
		return "更新失败", 100
	case "idle":
		return "未开始", 0
	default:
		return "启动任务", 8
	}
}

func tailLines(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}

func updateContainerRunning() bool {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", updateContainerName)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func detectUpdateCapability(d *Deps) updateCapability {
	capability := updateCapability{
		HostDir: updateHostDir(),
		Image:   updateImage(),
	}
	if !updateEnabled() {
		capability.Reason = "网页一键更新已关闭：设置 UPSTREAMHUB_UPDATE_ENABLED=true 后可启用"
		return capability
	}
	if d == nil || d.Auth == nil {
		capability.Reason = "网页一键更新要求开启登录鉴权，避免未登录用户触发服务器部署"
		return capability
	}
	if capability.HostDir == "" {
		capability.Reason = "缺少 UPSTREAMHUB_UPDATE_HOST_DIR，后端不知道宿主机上的项目目录"
		return capability
	}
	if _, err := exec.LookPath("docker"); err != nil {
		capability.Reason = "当前运行环境缺少 docker CLI，请重新部署包含更新能力的镜像"
		return capability
	}
	socket := dockerSocketPath()
	info, err := os.Stat(socket)
	if err != nil {
		capability.Reason = "当前容器无法访问 Docker socket，无法替你重建服务"
		return capability
	}
	if info.IsDir() {
		capability.Reason = "Docker socket 路径不是文件"
		return capability
	}
	capability.Available = true
	return capability
}

func dockerUpdateArgs(hostDir, socket, image, logFile string) []string {
	workDir := cleanDockerPath(hostDir)
	script := fmt.Sprintf(`set -eu
mkdir -p backups
{
  echo "[UPSTREAMHUB_STAGE] start|Updater container started"
  echo "[INFO] updater started at $(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
  echo "[INFO] workdir: $(pwd)"
  if bash ./scripts/deploy.sh; then
    echo "[UPSTREAMHUB_STAGE] done|Update completed"
    echo "[ OK ] updater finished at $(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
  else
    code=$?
    echo "[UPSTREAMHUB_STAGE] failed|Update failed with exit code ${code}"
    echo "[FAIL] updater failed at $(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
    exit ${code}
  fi
} > %s 2>&1`, shellQuote(logFile))
	return []string{
		"run",
		"-d",
		"--rm",
		"--name", updateContainerName,
		"--mount", "type=bind,source=" + socket + ",target=/var/run/docker.sock",
		"--mount", "type=bind,source=" + workDir + ",target=" + workDir,
		"-w", workDir,
		"--entrypoint", "/bin/sh",
		image,
		"-lc", script,
	}
}

func updateEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("UPSTREAMHUB_UPDATE_ENABLED"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func updateHostDir() string {
	if v := strings.TrimSpace(os.Getenv("UPSTREAMHUB_UPDATE_HOST_DIR")); v != "" {
		return cleanDockerPath(v)
	}
	for _, candidate := range []string{".", ".."} {
		if _, err := os.Stat(filepath.Join(candidate, "scripts", "deploy.sh")); err == nil {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
		}
	}
	return ""
}

func cleanDockerPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(path, "/") {
		for strings.Contains(path, "//") {
			path = strings.ReplaceAll(path, "//", "/")
		}
		return path
	}
	return filepath.Clean(path)
}

func updateImage() string {
	if v := strings.TrimSpace(os.Getenv("UPSTREAMHUB_UPDATE_IMAGE")); v != "" {
		return v
	}
	return defaultUpdateImage
}

func dockerSocketPath() string {
	if v := strings.TrimSpace(os.Getenv("UPSTREAMHUB_UPDATE_DOCKER_SOCKET")); v != "" {
		return v
	}
	return defaultDockerSocket
}

func sanitizeCommandOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1200 {
		s = s[len(s)-1200:]
	}
	return s
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
