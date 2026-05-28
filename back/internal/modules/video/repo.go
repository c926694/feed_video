package video

import (
	mysqlrepo "simple_tiktok/internal/repository/mysql"

	"gorm.io/gorm"
)

type VideoRepo = mysqlrepo.VideoRepo
type UserRepo = mysqlrepo.UserRepo
type CommentRepo = mysqlrepo.CommentRepo

func NewVideoRepo(db *gorm.DB) *VideoRepo {
	return mysqlrepo.NewVideoRepo(db)
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return mysqlrepo.NewUserRepo(db)
}

func NewCommentRepo(db *gorm.DB) *CommentRepo {
	return mysqlrepo.NewCommentRepo(db)
}
