package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

func init() {
	Register(storage.NotifyServerChan, func(raw string) (Notifier, error) { return newServerChan(raw) })
}

// sctpKeyRe 匹配 Server酱³ 的 sendkey 前缀，从中提取 uid。
// 形如 sctp1234t... → uid=1234。
var sctpKeyRe = regexp.MustCompile(`^sctp(\d+)t`)

type serverChanConfig struct {
	SendKey string `json:"sendkey"`
	// APIBase 可选：自建 / 独立域名时填完整前缀（如 https://xxx.example.com）。
	// 留空则按 sendkey 前缀自动选择 Turbo 版或 Server酱³ 官方地址。
	APIBase string `json:"api_base"`
}

type serverChan struct {
	cfg  serverChanConfig
	http *resty.Client
}

func newServerChan(raw string) (*serverChan, error) {
	var cfg serverChanConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	cfg.SendKey = strings.TrimSpace(cfg.SendKey)
	cfg.APIBase = strings.TrimSpace(cfg.APIBase)
	if cfg.SendKey == "" {
		return nil, errors.New("serverchan sendkey is required")
	}
	return &serverChan{cfg: cfg, http: resty.New()}, nil
}

func (s *serverChan) Type() storage.NotificationChannelType { return storage.NotifyServerChan }

// resolveServerChanURL 按官方 SDK 的规则决定推送地址：
//   - api_base 非空 → 自建域名，拼成 <api_base>/<sendkey>.send
//   - sendkey 以 sctp<数字>t 开头 → Server酱³：https://<uid>.push.ft07.com/send/<sendkey>.send
//   - 其它 → Turbo 版：https://sctapi.ftqq.com/<sendkey>.send
func resolveServerChanURL(sendkey, apiBase string) string {
	if apiBase != "" {
		return strings.TrimRight(apiBase, "/") + "/" + sendkey + ".send"
	}
	if m := sctpKeyRe.FindStringSubmatch(sendkey); m != nil {
		uid := m[1]
		return fmt.Sprintf("https://%s.push.ft07.com/send/%s.send", uid, sendkey)
	}
	return "https://sctapi.ftqq.com/" + sendkey + ".send"
}

// serverChanResp Server酱 返回体（两代字段一致）：成功 code==0。
type serverChanResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *serverChan) Send(ctx context.Context, msg Message) error {
	url := resolveServerChanURL(s.cfg.SendKey, s.cfg.APIBase)
	resp, err := s.http.R().
		SetContext(ctx).
		SetFormData(map[string]string{
			"title": msg.Subject,
			"desp":  msg.Body,
		}).
		Post(url)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return errors.New("serverchan returned " + resp.Status())
	}
	// 业务层失败：HTTP 200 但 code != 0，把 message 带出来便于排查。
	var body serverChanResp
	if err := json.Unmarshal(resp.Body(), &body); err == nil && body.Code != 0 {
		if body.Message != "" {
			return fmt.Errorf("serverchan error %d: %s", body.Code, body.Message)
		}
		return fmt.Errorf("serverchan error %d", body.Code)
	}
	return nil
}
