package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/model"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/pkg/util"
	mysql2 "simple_tiktok/internal/repository/mysql"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type CommentService struct {
	commentRepo *mysql2.CommentRepo
	videoRepo   *mysql2.VideoRepo
	userRepo    *mysql2.UserRepo
	redisClient *redis.Client
	hotMQ       *amqp.Channel
}

func NewCommentService(
	commentRepo *mysql2.CommentRepo,
	videoRepo *mysql2.VideoRepo,
	userRepo *mysql2.UserRepo,
	redisClient *redis.Client,
	hotMQ *amqp.Channel,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		videoRepo:   videoRepo,
		userRepo:    userRepo,
		redisClient: redisClient,
		hotMQ:       hotMQ,
	}
}

func (s *CommentService) CreateComment(id uint64, req req.CommentReq) (res.CommentRes, error) {
	tx := s.commentRepo.DB().Begin()
	if tx.Error != nil {
		return res.CommentRes{}, tx.Error
	}
	commentRepoTx := s.commentRepo.WithTx(tx)
	videoRepoTx := s.videoRepo.WithTx(tx)

	comment := &model.Comment{
		Content:   req.Content,
		VideoID:   req.VideoId,
		Commenter: id,
	}
	err := commentRepoTx.Save(comment)
	if err != nil {
		_ = tx.Rollback().Error
		return res.CommentRes{}, err
	}
	//更新comment_count
	err = videoRepoTx.UpdateCommentCount(comment.VideoID)
	if err != nil {
		_ = tx.Rollback().Error
		return res.CommentRes{}, err
	}
	if err = tx.Commit().Error; err != nil {
		return res.CommentRes{}, err
	}
	s.invalidateVideoInfoCache(comment.VideoID)
	s.publishVideoHotEvent(comment.VideoID, 1)
	commentRes := res.CommentRes{
		Id:        comment.ID,
		VideoId:   comment.VideoID,
		Commenter: comment.Commenter,
		Content:   comment.Content,
		LikeCount: comment.LikeCount,
		CreatedAt: comment.CreatedAt,
	}
	return commentRes, nil
}

func (s *CommentService) DeleteComment(userId uint64, id uint64) error {
	comment, err := s.commentRepo.GetById(id)
	if err != nil {
		return err
	}
	if comment.Commenter != userId {
		return errors.New("无法删除他人评论")
	}
	tx := s.commentRepo.DB().Begin()
	if tx.Error != nil {
		return tx.Error
	}
	commentRepoTx := s.commentRepo.WithTx(tx)
	videoRepoTx := s.videoRepo.WithTx(tx)

	err = commentRepoTx.DeleteComment(id)
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
	s.invalidateVideoInfoCache(comment.VideoID)
	s.publishVideoHotEvent(comment.VideoID, -1)
	return nil
}

func (s *CommentService) ListByVideoId(videoId uint64, userId uint64) ([]res.CommentRes, error) {
	commentList, err := s.commentRepo.ListByVideoId(videoId)
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
		isLiked, likeErr := s.redisClient.SIsMember(context.Background(), likeKey, userId).Result()
		if likeErr != nil {
			return nil, likeErr
		}
		commentRes := res.CommentRes{
			Id:        comment.ID,
			VideoId:   comment.VideoID,
			Commenter: comment.Commenter,
			Content:   comment.Content,
			LikeCount: comment.LikeCount,
			IsLiked:   isLiked,
			Author:    author,
			CreatedAt: comment.CreatedAt,
		}
		commentResList = append(commentResList, commentRes)
	}
	return commentResList, nil
}

func (s *CommentService) publishVideoHotEvent(videoId uint64, delta float64) {
	if s.hotMQ == nil {
		return
	}
	data, err := json.Marshal(event.VideoHotEvent{
		VideoId:     videoId,
		ScoreDelta:  delta,
		MinuteStamp: time.Now().UTC().Truncate(time.Minute).Unix(),
	})
	if err != nil {
		log.Println(err)
		return
	}
	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	}
	if err := s.hotMQ.Publish(event.VideoHotExchange, event.VideoHotRoutingKey, false, false, msg); err != nil {
		log.Println(err)
	}
}

func (s *CommentService) invalidateVideoInfoCache(videoID uint64) {
	cacheKey := fmt.Sprintf(constants.VideoInfoCacheKey, videoID)
	_ = s.redisClient.Del(context.Background(), cacheKey).Err()
}
