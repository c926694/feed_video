package comment

import (
	mysqlrepo "simple_tiktok/internal/repository/mysql"

	"gorm.io/gorm"
)

type CommentRepo = mysqlrepo.CommentRepo
type VideoRepo = mysqlrepo.VideoRepo
type UserRepo = mysqlrepo.UserRepo

func NewCommentRepo(db *gorm.DB) *CommentRepo {
	return mysqlrepo.NewCommentRepo(db)
}

func NewVideoRepo(db *gorm.DB) *VideoRepo {
	return mysqlrepo.NewVideoRepo(db)
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return mysqlrepo.NewUserRepo(db)
}
