package comment

import (
	"context"
	"errors"
	"fmt"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/model"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/pkg/util"
	mysql2 "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"

	"github.com/redis/go-redis/v9"
)

type Service struct {
	commentRepo *mysql2.CommentRepo
	videoRepo   *mysql2.VideoRepo
	userRepo    *mysql2.UserRepo
	redisClient *redis.Client
	feedService *service.FeedService
}

func NewService(
	commentRepo *mysql2.CommentRepo,
	videoRepo *mysql2.VideoRepo,
	userRepo *mysql2.UserRepo,
	redisClient *redis.Client,
	feedService *service.FeedService,
) *Service {
	return &Service{
		commentRepo: commentRepo,
		videoRepo:   videoRepo,
		userRepo:    userRepo,
		redisClient: redisClient,
		feedService: feedService,
	}
}

func (s *Service) CreateComment(userID uint64, commentReq req.CommentReq) (res.CommentRes, error) {
	tx := s.commentRepo.DB().Begin()
	if tx.Error != nil {
		return res.CommentRes{}, tx.Error
	}
	commentRepoTx := s.commentRepo.WithTx(tx)
	videoRepoTx := s.videoRepo.WithTx(tx)

	comment := &model.Comment{
		Content:   commentReq.Content,
		VideoID:   commentReq.VideoId,
		Commenter: userID,
	}
	err := commentRepoTx.Save(comment)
	if err != nil {
		_ = tx.Rollback().Error
		return res.CommentRes{}, err
	}
	err = videoRepoTx.UpdateCommentCount(comment.VideoID)
	if err != nil {
		_ = tx.Rollback().Error
		return res.CommentRes{}, err
	}
	if err = tx.Commit().Error; err != nil {
		return res.CommentRes{}, err
	}
	if s.feedService != nil {
		s.feedService.MustInvalidateVideoInfoCache(comment.VideoID)
		if err = s.feedService.PublishVideoHotEvent(comment.VideoID, 1); err != nil {
			return res.CommentRes{}, err
		}
	}
	return res.CommentRes{
		Id:        comment.ID,
		VideoId:   comment.VideoID,
		Commenter: comment.Commenter,
		Content:   comment.Content,
		LikeCount: comment.LikeCount,
		CreatedAt: comment.CreatedAt,
	}, nil
}

func (s *Service) DeleteComment(userID uint64, commentID uint64) error {
	comment, err := s.commentRepo.GetById(commentID)
	if err != nil {
		return err
	}
	if comment.Commenter != userID {
		return errors.New("无法删除他人评论")
	}
	tx := s.commentRepo.DB().Begin()
	if tx.Error != nil {
		return tx.Error
	}
	commentRepoTx := s.commentRepo.WithTx(tx)
	videoRepoTx := s.videoRepo.WithTx(tx)

	err = commentRepoTx.DeleteComment(commentID)
	if err != nil {
		_ = tx.Rollback().Error
		return err
	}
	err = videoRepoTx.DeleteCommentCount(comment.VideoID)
	if err != nil {
		_ = tx.Rollback().Error
		return err
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	if s.feedService != nil {
		s.feedService.MustInvalidateVideoInfoCache(comment.VideoID)
		if err = s.feedService.PublishVideoHotEvent(comment.VideoID, -1); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListByVideoId(videoID uint64, userID uint64) ([]res.CommentRes, error) {
	commentList, err := s.commentRepo.ListByVideoId(videoID)
	if err != nil {
		return nil, err
	}
	commentResList := make([]res.CommentRes, 0, len(commentList))
	authorCache := make(map[uint64]res.UserInfoRes)
	for _, comment := range commentList {
		author, ok := authorCache[comment.Commenter]
		if !ok {
			user, userErr := s.userRepo.GetUserByID(comment.Commenter)
			if userErr == nil && user != nil {
				author = res.UserInfoRes{
					UserID:        user.ID,
					Username:      user.Username,
					Nickname:      user.NickName,
					AvatarURL:     util.EnsureHTTPPath(user.AvatarURL),
					FollowCount:   user.FollowCount,
					FollowerCount: user.FollowerCount,
				}
			} else {
				author = res.UserInfoRes{
					UserID:    comment.Commenter,
					Username:  "anonymous",
					Nickname:  "匿名用户",
					AvatarURL: "",
				}
			}
			authorCache[comment.Commenter] = author
		}
		likeKey := fmt.Sprintf(constants.LikeComment, comment.ID)
		isLiked, likeErr := s.redisClient.SIsMember(context.Background(), likeKey, userID).Result()
		if likeErr != nil {
			return nil, likeErr
		}
		commentResList = append(commentResList, res.CommentRes{
			Id:        comment.ID,
			VideoId:   comment.VideoID,
			Commenter: comment.Commenter,
			Content:   comment.Content,
			LikeCount: comment.LikeCount,
			IsLiked:   isLiked,
			Author:    author,
			CreatedAt: comment.CreatedAt,
		})
	}
	return commentResList, nil
}
