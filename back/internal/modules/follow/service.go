package follow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/mq/event"
	"simple_tiktok/internal/pkg/constants"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	redisClient *redis.Client
	followMQ    *amqp.Channel
}

var switchFollowScript = redis.NewScript(`
if redis.call("SISMEMBER", KEYS[1], ARGV[1]) == 1 then
    redis.call("SREM", KEYS[1], ARGV[1])
    return 0
end
redis.call("SADD", KEYS[1], ARGV[1])
return 1
`)

func NewService(redisClient *redis.Client, followMQ *amqp.Channel) *Service {
	return &Service{
		redisClient: redisClient,
		followMQ:    followMQ,
	}
}

func (s *Service) Follow(targetUserID uint64, currentUserID uint64) (res.FollowRes, error) {
	if targetUserID == currentUserID {
		return res.FollowRes{}, errors.New("cannot follow yourself")
	}
	key := fmt.Sprintf(constants.FollowKey, currentUserID)
	followed, err := s.switchFollow(context.Background(), key, targetUserID)
	if err != nil {
		return res.FollowRes{}, err
	}

	eventType := event.Unfollow
	rollback := func() error {
		return s.redisClient.SAdd(context.Background(), key, targetUserID).Err()
	}
	if followed {
		eventType = event.Follow
		rollback = func() error {
			return s.redisClient.SRem(context.Background(), key, targetUserID).Err()
		}
	}

	msg, err := s.getFollowEventMsg(targetUserID, currentUserID, eventType)
	if err != nil {
		return res.FollowRes{}, err
	}

	log.Printf("follow switch follower=%d following=%d followed=%t", currentUserID, targetUserID, followed)
	err = s.followMQ.Publish(event.FollowExchange, event.FollowRoutingKey, false, false, msg)
	if err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			log.Printf("follow publish failed and rollback failed follower=%d following=%d err=%v rollback_err=%v", currentUserID, targetUserID, err, rollbackErr)
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

func (s *Service) getFollowEventMsg(following uint64, follower uint64, eventType string) (amqp.Publishing, error) {
	e := event.FollowEvent{Following: following, Follower: follower, EventType: eventType}
	data, err := json.Marshal(e)
	if err != nil {
		return amqp.Publishing{}, err
	}
	return amqp.Publishing{ContentType: "application/json", Body: data}, nil
}
