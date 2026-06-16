package storage

import (
	"errors"

	"gorm.io/gorm"
)

type AdminUsers struct{ db *gorm.DB }

func NewAdminUsers(db *gorm.DB) *AdminUsers { return &AdminUsers{db: db} }

// FindByUsername 按用户名取管理员。返回 (nil, nil) 表示不存在。
func (r *AdminUsers) FindByUsername(username string) (*AdminUser, error) {
	var u AdminUser
	err := r.db.First(&u, "username = ?", username).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Count 返回管理员总数，供启动时判断是否需要 seed 默认账号。
func (r *AdminUsers) Count() (int64, error) {
	var n int64
	if err := r.db.Model(&AdminUser{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// Create 新建管理员。
func (r *AdminUsers) Create(u *AdminUser) error { return r.db.Create(u).Error }

// UpdatePassword 更新某管理员的密码哈希并清除"强制改密"标志。
func (r *AdminUsers) UpdatePassword(id uint, passwordHash string) error {
	return r.db.Model(&AdminUser{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"password_hash":        passwordHash,
			"must_change_password": false,
		}).Error
}
