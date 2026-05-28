package feed

import (
	mysqlrepo "simple_tiktok/internal/repository/mysql"

	"gorm.io/gorm"
)

type VideoRepo = mysqlrepo.VideoRepo
type UserRepo = mysqlrepo.UserRepo

func NewVideoRepo(db *gorm.DB) *VideoRepo {
	return mysqlrepo.NewVideoRepo(db)
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return mysqlrepo.NewUserRepo(db)
}
