package follow

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/redis/go-redis/v9"

	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/platform/kafka"
)

type Service struct {
	redisClient *redis.Client
	publisher   *kafka.Publisher
}

var switchFollowScript = redis.NewScript(`
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

func (s *Service) Follow(ctx context.Context, targetUserID uint64, currentUserID uint64) (res.FollowRes, error) {
	if targetUserID == currentUserID {
		return res.FollowRes{}, httpx.New(httpx.CodeBadRequest, "不能关注自己")
	}
	key := fmt.Sprintf(constants.FollowKey, currentUserID)
	followed, err := s.switchFollow(ctx, key, targetUserID)
	if err != nil {
		return res.FollowRes{}, err
	}

	eventType := event.Unfollow
	rollback := func() error {
		return s.redisClient.SAdd(ctx, key, targetUserID).Err()
	}
	if followed {
		eventType = event.Follow
		rollback = func() error {
			return s.redisClient.SRem(ctx, key, targetUserID).Err()
		}
	}

	err = s.publisher.Publish(ctx, kafka.TopicFollow, strconv.FormatUint(targetUserID, 10), event.FollowEvent{
		Following: targetUserID,
		Follower:  currentUserID,
		EventType: eventType,
	})
	if err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			slog.Error("关注事件发布失败且状态回滚失败",
				"follower", currentUserID,
				"following", targetUserID,
				"error", err,
				"rollback_error", rollbackErr)
		}
		return res.FollowRes{}, err
	}
	return res.FollowRes{
		Following: targetUserID,
		IsFollow:  followed,
	}, nil
}

func (s *Service) switchFollow(ctx context.Context, key string, following uint64) (bool, error) {
	result, err := switchFollowScript.Run(ctx, s.redisClient, []string{key}, following).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
