package like

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/pkg/constants"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

type Service struct {
	redisClient       *redis.Client
	likeVideoWriter   *kafka.Writer
	likeCommentWriter *kafka.Writer
}

var switchLikeScript = redis.NewScript(`
if redis.call("SISMEMBER", KEYS[1], ARGV[1]) == 1 then
    redis.call("SREM", KEYS[1], ARGV[1])
    return 0
end
redis.call("SADD", KEYS[1], ARGV[1])
return 1
`)

func NewService(redisClient *redis.Client, likeVideoWriter *kafka.Writer, likeCommentWriter *kafka.Writer) *Service {
	return &Service{
		redisClient:       redisClient,
		likeVideoWriter:   likeVideoWriter,
		likeCommentWriter: likeCommentWriter,
	}
}

func (s *Service) LikeVideo(targetID uint64, userID uint64) (res.LikeVideoRes, error) {
	key := fmt.Sprintf(constants.LikeVideo, targetID)
	liked, err := s.switchLike(context.Background(), key, userID)
	if err != nil {
		return res.LikeVideoRes{}, err
	}

	eventType := event.Dislike
	rollback := func() error {
		return s.redisClient.SAdd(context.Background(), key, userID).Err()
	}
	if liked {
		eventType = event.Like
		rollback = func() error {
			return s.redisClient.SRem(context.Background(), key, userID).Err()
		}
	}

	msgData, err := s.getLikeVideoEventMsg(targetID, eventType)
	if err != nil {
		return res.LikeVideoRes{}, err
	}

	log.Printf("like switch video_id=%d user_id=%d liked=%t", targetID, userID, liked)
	err = s.likeVideoWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", targetID)),
		Value: msgData,
	})
	if err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			log.Printf("like publish failed and rollback failed video_id=%d user_id=%d err=%v rollback_err=%v", targetID, userID, err, rollbackErr)
		}
		return res.LikeVideoRes{}, err
	}

	return res.LikeVideoRes{VideoId: targetID, IsLiked: liked}, nil
}

func (s *Service) LikeComment(commentID uint64, userID uint64) (res.LikeCommentRes, error) {
	key := fmt.Sprintf(constants.LikeComment, commentID)
	liked, err := s.switchLike(context.Background(), key, userID)
	if err != nil {
		return res.LikeCommentRes{}, err
	}

	eventType := event.Dislike
	rollback := func() error {
		return s.redisClient.SAdd(context.Background(), key, userID).Err()
	}
	if liked {
		eventType = event.Like
		rollback = func() error {
			return s.redisClient.SRem(context.Background(), key, userID).Err()
		}
	}

	msgData, err := s.getLikeCommentEventMsg(commentID, eventType)
	if err != nil {
		return res.LikeCommentRes{}, err
	}

	log.Printf("comment like switch comment_id=%d user_id=%d liked=%t", commentID, userID, liked)
	err = s.likeCommentWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", commentID)),
		Value: msgData,
	})
	if err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			log.Printf("comment like publish failed and rollback failed comment_id=%d user_id=%d err=%v rollback_err=%v", commentID, userID, err, rollbackErr)
		}
		return res.LikeCommentRes{}, err
	}

	return res.LikeCommentRes{CommentId: commentID, IsLiked: liked}, nil
}

func (s *Service) switchLike(ctx context.Context, key string, userID uint64) (bool, error) {
	result, err := switchLikeScript.Run(ctx, s.redisClient, []string{key}, userID).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *Service) getLikeVideoEventMsg(videoID uint64, eventType string) ([]byte, error) {
	e := event.LikeVideoEvent{VideoId: videoID, EventType: eventType}
	return json.Marshal(e)
}

func (s *Service) getLikeCommentEventMsg(commentID uint64, eventType string) ([]byte, error) {
	e := event.LikeCommentEvent{CommentId: commentID, EventType: eventType}
	return json.Marshal(e)
}
