package follow

import (
	mysqlrepo "simple_tiktok/internal/repository/mysql"

	"gorm.io/gorm"
)

type FollowRepo = mysqlrepo.FollowRepo
type UserRepo = mysqlrepo.UserRepo

func NewFollowRepo(db *gorm.DB) *FollowRepo {
	return mysqlrepo.NewFollowRepo(db)
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return mysqlrepo.NewUserRepo(db)
}
