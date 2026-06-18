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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	updateContainerName = "upstreamhub-updater"
	defaultUpdateImage  = "upstream-hub:local"
	defaultDockerSocket = "/var/run/docker.sock"
)

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
  echo "[INFO] updater started at $(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
  echo "[INFO] workdir: $(pwd)"
  bash ./scripts/deploy.sh
  echo "[ OK ] updater finished at $(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
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
