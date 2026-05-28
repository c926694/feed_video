package user

import (
	mysqlrepo "simple_tiktok/internal/repository/mysql"

	"gorm.io/gorm"
)

type UserRepo = mysqlrepo.UserRepo
type VideoRepo = mysqlrepo.VideoRepo

func NewUserRepo(db *gorm.DB) *UserRepo {
	return mysqlrepo.NewUserRepo(db)
}

func NewVideoRepo(db *gorm.DB) *VideoRepo {
	return mysqlrepo.NewVideoRepo(db)
}
