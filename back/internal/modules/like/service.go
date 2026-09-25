package like

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/redis/go-redis/v9"

	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/platform/kafka"
)

type Service struct {
	redisClient *redis.Client
	publisher   *kafka.Publisher
}

var switchLikeScript = redis.NewScript(`
if redis.call("SISMEMBER", KEYS[1], ARGV[1]) == 1 then
    redis.call("SREM", KEYS[1], ARGV[1])
    return 0
end
redis.call("SADD", KEYS[1], ARGV[1])
return 1
`)

func NewService(redisClient *redis.Client, publisher *kafka.Publisher) *Service {
	return &Service{
		redisClient: redisClient,
		publisher:   publisher,
	}
}

func (s *Service) LikeVideo(ctx context.Context, targetID uint64, userID uint64) (res.LikeVideoRes, error) {
	key := fmt.Sprintf(constants.LikeVideo, targetID)
	liked, err := s.switchLike(ctx, key, userID)
	if err != nil {
		return res.LikeVideoRes{}, err
	}

	eventType := event.Dislike
	rollback := func() error {
		return s.redisClient.SAdd(ctx, key, userID).Err()
	}
	if liked {
		eventType = event.Like
		rollback = func() error {
			return s.redisClient.SRem(ctx, key, userID).Err()
		}
	}

	err = s.publisher.Publish(ctx, kafka.TopicLikeVideo, strconv.FormatUint(targetID, 10), event.LikeVideoEvent{
		VideoId:   targetID,
		EventType: eventType,
	})
	if err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			slog.Error("点赞事件发布失败且状态回滚失败", "video_id", targetID, "user_id", userID, "error", err, "rollback_error", rollbackErr)
		}
		return res.LikeVideoRes{}, err
	}
	return res.LikeVideoRes{VideoId: targetID, IsLiked: liked}, nil
}

func (s *Service) LikeComment(ctx context.Context, commentID uint64, userID uint64) (res.LikeCommentRes, error) {
	key := fmt.Sprintf(constants.LikeComment, commentID)
	liked, err := s.switchLike(ctx, key, userID)
	if err != nil {
		return res.LikeCommentRes{}, err
	}

	eventType := event.Dislike
	rollback := func() error {
		return s.redisClient.SAdd(ctx, key, userID).Err()
	}
	if liked {
		eventType = event.Like
		rollback = func() error {
			return s.redisClient.SRem(ctx, key, userID).Err()
		}
	}

	err = s.publisher.Publish(ctx, kafka.TopicLikeComment, strconv.FormatUint(commentID, 10), event.LikeCommentEvent{
		CommentId: commentID,
		EventType: eventType,
	})
	if err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			slog.Error("评论点赞事件发布失败且状态回滚失败", "comment_id", commentID, "user_id", userID, "error", err, "rollback_error", rollbackErr)
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
