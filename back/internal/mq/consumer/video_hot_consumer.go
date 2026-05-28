package consumer

import (
	"encoding/json"
	"errors"
	"log"
	"simple_tiktok/internal/mq/event"
	mysql2 "simple_tiktok/internal/repository/mysql"
	"simple_tiktok/internal/service"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type VideoHotConsumer struct {
	channel      *amqp.Channel
	videoRepo    *mysql2.VideoRepo
	feedService  *service.FeedService
}

func NewVideoHotConsumer(channel *amqp.Channel, videoRepo *mysql2.VideoRepo, feedService *service.FeedService) *VideoHotConsumer {
	return &VideoHotConsumer{
		channel:      channel,
		videoRepo:    videoRepo,
		feedService:  feedService,
	}
}

func (c *VideoHotConsumer) Declare(exchange string, exchangeType string, queue string, routingKey string) error {
	if c == nil || c.channel == nil {
		return errors.New("consumer channel is nil")
	}

	if err := c.channel.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil); err != nil {
		return err
	}

	declaredQueue, err := c.channel.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	return c.channel.QueueBind(declaredQueue.Name, routingKey, exchange, false, nil)
}

func (c *VideoHotConsumer) Listen(queue string, handler func(amqp.Delivery)) error {
	if c == nil || c.channel == nil {
		return errors.New("consumer channel is nil")
	}

	deliveries, err := c.channel.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for delivery := range deliveries {
		handler(delivery)
	}

	return nil
}

func (c *VideoHotConsumer) HotUpdateHandler(msg amqp.Delivery) {
	if c == nil || c.channel == nil {
		log.Println("consumer channel is nil")
		_ = msg.Nack(false, false)
		return
	}

	var e event.VideoHotEvent
	if err := json.Unmarshal(msg.Body, &e); err != nil {
		log.Println(err)
		_ = msg.Nack(false, false)
		return
	}

	videoId := e.VideoId
	if e.ScoreDelta == 0 {
		_ = msg.Ack(false)
		return
	}
	video, err := c.videoRepo.GetVideoById(videoId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = msg.Ack(false)
			return
		}
		log.Println(err)
		_ = msg.Nack(false, true)
		return
	}

	minute := time.Now()
	if e.MinuteStamp > 0 {
		minute = time.Unix(e.MinuteStamp, 0)
	}
	if err := c.feedService.IncrementHotScoreByMinute(video.ID, e.ScoreDelta, minute); err != nil {
		log.Println(err)
		_ = msg.Nack(false, true)
		return
	}

	_ = msg.Ack(false)
}
