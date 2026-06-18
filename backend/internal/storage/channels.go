package storage

import (
	"gorm.io/gorm"
)

// Channels 渠道仓库。
type Channels struct{ db *gorm.DB }

func NewChannels(db *gorm.DB) *Channels { return &Channels{db: db} }

func (r *Channels) Create(c *Channel) error { return r.db.Create(c).Error }
func (r *Channels) Update(c *Channel) error { return r.db.Save(c).Error }
func (r *Channels) Delete(id uint) error    { return r.db.Delete(&Channel{}, id).Error }
func (r *Channels) FindByID(id uint) (*Channel, error) {
	var c Channel
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}
func (r *Channels) List() ([]Channel, error) {
	var list []Channel
	if err := r.db.Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
func (r *Channels) ListMonitorEnabled() ([]Channel, error) {
	var list []Channel
	if err := r.db.Where("monitor_enabled = ?", true).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
func (r *Channels) CountByCaptchaConfig(id uint) (int64, error) {
	var n int64
	err := r.db.Model(&Channel{}).Where("captcha_config_id = ?", id).Count(&n).Error
	return n, err
}
func (r *Channels) UpdateBalance(id uint, balance float64, at any, lastErr string) error {
	return r.db.Model(&Channel{}).Where("id = ?", id).Updates(map[string]any{
		"last_balance":    balance,
		"last_balance_at": at,
		"last_error":      lastErr,
	}).Error
}
func (r *Channels) SetLastError(id uint, msg string) error {
	return r.db.Model(&Channel{}).Where("id = ?", id).Update("last_error", msg).Error
}
