package test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/readr-media/readr-restful-following/config"
	"github.com/readr-media/readr-restful-following/internal/router"
)

var r *gin.Engine

type GenericTestcase struct {
	Name     string
	Method   string
	URL      string
	Body     interface{}
	Httpcode int
	Resp     interface{}
}

func SetRoutes(handlers []router.RouterHandler) {
	r = gin.New()
	for _, handler := range handlers {
		handler.SetRoutes(r)
	}
}

func InitHttpTest() {
	os.Setenv("mode", "local")
	os.Setenv("db_driver", "mock")

	if _, err := config.LoadConfig("../config"); err != nil {
		panic(fmt.Errorf("Invalid application configuration: %s", err))
	}

	// // Init Redis connetions
	// models.RedisConn(map[string]string{
	// 	"read_url":  fmt.Sprint(config.Config.Redis.ReadURL),
	// 	"write_url": fmt.Sprint(config.Config.Redis.WriteURL),
	// 	"password":  fmt.Sprint(config.Config.Redis.Password),
	// })

	// models.SearchFeed.Init(false)

	gin.SetMode(gin.TestMode)
}

func GenericDoTest(tc GenericTestcase, t *testing.T, function interface{}) {
	t.Run(tc.Name, func(t *testing.T) {
		w := httptest.NewRecorder()
		jsonStr := []byte{}
		if s, ok := tc.Body.(string); ok {
			jsonStr = []byte(s)
		} else {
			p, err := json.Marshal(tc.Body)
			if err != nil {
				t.Errorf("%s, Error when marshaling input parameters", tc.Name)
			}
			jsonStr = p
		}
		req, _ := http.NewRequest(tc.Method, tc.URL, bytes.NewBuffer(jsonStr))
		if tc.Method == "GET" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}

		r.ServeHTTP(w, req)

		if w.Code != tc.Httpcode {
			t.Errorf("%s want %d but get %d", tc.Name, tc.Httpcode, w.Code)
		}
		switch tc.Resp.(type) {
		case string:
			if w.Body.String() != tc.Resp {
				t.Errorf("%s expect (error) message %v but get %v", tc.Name, tc.Resp, w.Body.String())
			}
		default:
			if fn, ok := function.(func(resp string, tc GenericTestcase, t *testing.T)); ok {
				fn(w.Body.String(), tc, t)
			}
		}
	})
}
