package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/captcha"
	"github.com/worryzyy/upstream-hub/internal/storage"
)

func registerCaptchas(g *gin.RouterGroup, d *Deps) {
	gp := g.Group("/captcha-configs")
	gp.GET("", func(c *gin.Context) {
		list, err := d.Captchas.List()
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": list})
	})
	gp.POST("", func(c *gin.Context) { createCaptcha(c, d) })
	gp.PUT("/:id", func(c *gin.Context) { updateCaptcha(c, d) })
	gp.DELETE("/:id", func(c *gin.Context) {
		id, err := uintParam(c, "id")
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		n, err := d.Channels.CountByCaptchaConfig(id)
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		if n > 0 {
			fail(c, http.StatusConflict, errors.New("captcha config is still used by channels"))
			return
		}
		if err := d.Captchas.Delete(id); err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

type captchaInput struct {
	Name     string                      `json:"name"`
	Type     storage.CaptchaProviderType `json:"type"`
	APIKey   string                      `json:"api_key"`
	Endpoint string                      `json:"endpoint"`
	Extra    string                      `json:"extra"`
	Enabled  *bool                       `json:"enabled"`
}

func createCaptcha(c *gin.Context, d *Deps) {
	var in captchaInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.APIKey = strings.TrimSpace(in.APIKey)
	endpoint, err := normalizeOptionalHTTPURL(in.Endpoint, "endpoint")
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	in.Endpoint = endpoint
	if in.Name == "" {
		fail(c, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if in.Type == "" {
		fail(c, http.StatusBadRequest, errors.New("type is required"))
		return
	}
	if in.APIKey == "" {
		fail(c, http.StatusBadRequest, errors.New("api_key is required"))
		return
	}
	if err := validateCaptchaConfig(in, in.APIKey); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	key, err := d.Cipher.Encrypt(in.APIKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	cfg := &storage.CaptchaConfig{
		Name:         in.Name,
		Type:         in.Type,
		APIKeyCipher: key,
		Endpoint:     in.Endpoint,
		Extra:        in.Extra,
		Enabled:      enabled,
	}
	if err := d.Captchas.Create(cfg); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

func updateCaptcha(c *gin.Context, d *Deps) {
	id, err := uintParam(c, "id")
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	cfg, err := d.Captchas.FindByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, err)
		return
	}
	var in captchaInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	nextName := cfg.Name
	if strings.TrimSpace(in.Name) != "" {
		nextName = strings.TrimSpace(in.Name)
	}
	nextType := cfg.Type
	if in.Type != "" {
		if in.Type != cfg.Type {
			fail(c, http.StatusBadRequest, errors.New("captcha provider type cannot be changed after creation"))
			return
		}
		nextType = in.Type
	}
	nextEndpoint := cfg.Endpoint
	if in.Endpoint != "" {
		nextEndpoint, err = normalizeOptionalHTTPURL(in.Endpoint, "endpoint")
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
	}
	nextExtra := cfg.Extra
	if in.Extra != "" {
		nextExtra = in.Extra
	}
	if in.Enabled != nil {
		cfg.Enabled = *in.Enabled
	}
	apiKey := strings.TrimSpace(in.APIKey)
	if apiKey == "" {
		apiKey, err = d.Cipher.Decrypt(cfg.APIKeyCipher)
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
	}
	next := captchaInput{
		Name:     nextName,
		Type:     nextType,
		Endpoint: nextEndpoint,
		Extra:    nextExtra,
	}
	if err := validateCaptchaConfig(next, apiKey); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	cfg.Name = nextName
	cfg.Type = nextType
	cfg.Endpoint = nextEndpoint
	cfg.Extra = nextExtra
	if apiKey != "" && strings.TrimSpace(in.APIKey) != "" {
		key, err := d.Cipher.Encrypt(apiKey)
		if err != nil {
			fail(c, http.StatusInternalServerError, err)
			return
		}
		cfg.APIKeyCipher = key
	}
	if err := d.Captchas.Update(cfg); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

func validateCaptchaConfig(in captchaInput, apiKey string) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("api_key is required")
	}
	cfg := &storage.CaptchaConfig{
		Type:     in.Type,
		Endpoint: strings.TrimSpace(in.Endpoint),
		Extra:    in.Extra,
	}
	_, err := captcha.Build(cfg, apiKey)
	return err
}

func normalizeOptionalHTTPURL(raw, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", field)
	}
	return strings.TrimRight(raw, "/"), nil
}
