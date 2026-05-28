package like

import (
	mysqlrepo "simple_tiktok/internal/repository/mysql"

	"gorm.io/gorm"
)

type VideoRepo = mysqlrepo.VideoRepo
type CommentRepo = mysqlrepo.CommentRepo
type UserRepo = mysqlrepo.UserRepo

func NewVideoRepo(db *gorm.DB) *VideoRepo {
	return mysqlrepo.NewVideoRepo(db)
}

func NewCommentRepo(db *gorm.DB) *CommentRepo {
	return mysqlrepo.NewCommentRepo(db)
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return mysqlrepo.NewUserRepo(db)
}
