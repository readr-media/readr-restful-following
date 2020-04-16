package router

import (
	"fmt"
	"log"

	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/readr-media/readr-restful-following/config"
	"github.com/readr-media/readr-restful-following/pkg/following/model"
)

var supportedAction = map[string]bool{
	"follow":         true,
	"unfollow":       true,
	"insert_emotion": true,
	"update_emotion": true,
	"delete_emotion": true,
	"post_comment":   true,
	"edit_comment":   true,
	"delete_comment": true,
}

type PubsubMessageMetaBody struct {
	ID   string            `json:"messageId"`
	Body []byte            `json:"data"`
	Attr map[string]string `json:"attributes"`
}

type PubsubMessageMeta struct {
	Subscription string `json:"subscription"`
	Message      PubsubMessageMetaBody
}

type PubsubFollowMsgBody struct {
	Resource string `json:"resource"`
	Emotion  string `json:"emotion"`
	Subject  int    `json:"subject"`
	Object   int    `json:"object"`
}

type pubsubHandler struct{}

func (r *pubsubHandler) Push(c *gin.Context) {
	var (
		input PubsubMessageMeta
		err   error
	)
	c.ShouldBindJSON(&input)

	msgType := input.Message.Attr["type"]
	actionType := input.Message.Attr["action"]

	switch msgType {
	case "follow", "emotion":

		var body PubsubFollowMsgBody

		err = json.Unmarshal(input.Message.Body, &body)
		if err != nil {
			log.Printf("Parse msg body fail: %v \n", err.Error())
			c.JSON(http.StatusOK, gin.H{"Error": "Bad Request"})
			return
		}
		params := model.FollowArgs{Resource: body.Resource, Subject: int64(body.Subject), Object: int64(body.Object)}
		if val, ok := config.Config.Models.FollowingType[body.Resource]; ok {
			params.Type = val
		} else {
			c.JSON(http.StatusOK, gin.H{"Error": "Unsupported Resource"})
			return
		}

		if msgType == "follow" {

			// Follow situation set Emotion to none.
			if params.Emotion != 0 {
				params.Emotion = 0
			}

			switch actionType {
			case "follow":
				err = model.FollowingAPI.Insert(params)
			case "unfollow":
				err = model.FollowingAPI.Delete(params)
			default:
				log.Println("Follow action Type Not Support", actionType)
				c.JSON(http.StatusOK, gin.H{"Error": "Bad Request"})
				return
			}

		} else if msgType == "emotion" {

			// Rule out member
			if params.Resource == "member" {
				c.JSON(http.StatusOK, gin.H{"Error": "Emotion Not Available For Member"})
				return
			}
			if val, ok := config.Config.Models.Emotions[body.Emotion]; ok {
				params.Emotion = val
			} else {
				c.JSON(http.StatusOK, gin.H{"Error": "Unsupported Emotion"})
				return
			}

			switch actionType {
			case "insert":
				err = model.FollowingAPI.Insert(params)
			case "update":
				err = model.FollowingAPI.Update(params)
			case "delete":
				err = model.FollowingAPI.Delete(params)
			default:
				log.Printf("Emotion action Type %s Not Support", actionType)
				c.JSON(http.StatusOK, gin.H{"Error": "Bad Request"})
				return
			}
		}

		if err != nil {
			log.Printf("%s fail: %v\n", actionType, err.Error())
			c.JSON(http.StatusOK, gin.H{"Error": err.Error()})
			return
		}

		c.Status(http.StatusOK)

	default:
		log.Println("Pubsub Message Type Not Support", actionType)
		fmt.Println(msgType)
		c.Status(http.StatusOK)
		return
	}
}

func (r *pubsubHandler) SetRoutes(router *gin.Engine) {
	router.POST("/restful/pubsub", r.Push)
}

var PubsubRouter pubsubHandler
