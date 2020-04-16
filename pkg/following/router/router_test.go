package router

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"net/http"

	"github.com/readr-media/readr-restful-following/config"
	"github.com/readr-media/readr-restful-following/internal/router"
	tc "github.com/readr-media/readr-restful-following/internal/test"
	"github.com/readr-media/readr-restful-following/pkg/following/model"
)

type mockFollowingAPI struct{}

type followDS struct {
	ID     int64
	Object int64
}

var mockFollowingDS = map[string][]followDS{
	"post":    []followDS{},
	"member":  []followDS{},
	"project": []followDS{},
}

func (a *mockFollowingAPI) Get(params model.GetFollowInterface) (result interface{}, err error) {

	switch params := params.(type) {
	case *model.GetFollowingArgs:
		result, err = getFollowing(params)
	case *model.GetFollowedArgs:
		result, err = getFollowed(params)
	case *model.GetFollowerMemberIDsArgs:
		result, err = getFollowerMemberIDs(params)
	default:
		return nil, errors.New("Unsupported Query Args")
	}
	return result, err
}

func (a *mockFollowingAPI) Insert(params model.FollowArgs) error {

	store, ok := mockFollowingDS[params.Resource]
	if !ok {
		return errors.New("Resource Not Supported")
	}

	store = append(store, followDS{ID: params.Subject, Object: params.Object})
	return nil
}

func (a *mockFollowingAPI) Update(params model.FollowArgs) error {
	return nil
}

func (a *mockFollowingAPI) Delete(params model.FollowArgs) error {

	store, ok := mockFollowingDS[params.Resource]
	if !ok {
		return errors.New("Resource Not Supported")
	}
	for index, follow := range store {
		if follow.ID == params.Subject && follow.Object == params.Object {
			store = append(store[:index], store[index+1:]...)
		}
	}
	return nil
}

func getFollowing(params *model.GetFollowingArgs) (followings []interface{}, err error) {
	fmt.Println("params", params)
	switch {
	case params.MemberID == 0:
		return nil, errors.New("Not Found")
	default:
		return nil, nil
	}
	return nil, nil
}

func getFollowed(args *model.GetFollowedArgs) (interface{}, error) {

	switch {
	case args.IDs[0] == 1001:
		return []model.FollowedCount{}, nil
	case args.ResourceName == "member":
		return []model.FollowedCount{
			model.FollowedCount{71, 1, []int64{72}},
			model.FollowedCount{72, 1, []int64{71}},
		}, nil
	case args.ResourceName == "post":
		switch args.ResourceType {
		case "":
			return []model.FollowedCount{
				model.FollowedCount{42, 2, []int64{71, 72}},
				model.FollowedCount{84, 1, []int64{71}},
			}, nil
		case "review":
			return []model.FollowedCount{
				model.FollowedCount{42, 2, []int64{71, 72}},
			}, nil
		case "news":
			return []model.FollowedCount{
				model.FollowedCount{84, 1, []int64{71}},
			}, nil
		}
		return nil, nil
	case args.ResourceName == "project":
		switch len(args.IDs) {
		case 1:
			return []model.FollowedCount{
				model.FollowedCount{840, 1, []int64{72}},
			}, nil
		case 2:
			return []model.FollowedCount{
				model.FollowedCount{420, 2, []int64{71, 72}},
				model.FollowedCount{840, 1, []int64{72}},
			}, nil
		}
		return nil, nil
	default:
		return nil, nil
	}
}

func getFollowerMemberIDs(args *model.GetFollowerMemberIDsArgs) ([]int, error) {
	return []int{}, nil
}

type mockFollowCache struct{}

func (m mockFollowCache) Update(i model.GetFollowedArgs, f []model.FollowedCount)              {}
func (m mockFollowCache) Revoke(actionType string, resource string, emotion int, object int64) {}

func TestMain(m *testing.M) {

	_, err := config.LoadConfig("../../../config/main.json")
	if err != nil {
		panic(fmt.Errorf("Invalid application configuration: %s", err))
	}

	tc.SetRoutes([]router.RouterHandler{&Router, &PubsubRouter})

	model.FollowingAPI = new(mockFollowingAPI)

	os.Exit(m.Run())
}

func TestFollowing(t *testing.T) {

	transformPubsub := func(testcase tc.GenericTestcase) tc.GenericTestcase {
		meta := PubsubMessageMeta{
			Subscription: "sub",
			Message: PubsubMessageMetaBody{
				ID:   "1",
				Body: []byte(testcase.Body.(string)),
				Attr: map[string]string{"type": "follow", "action": testcase.Method},
			},
		}

		return tc.GenericTestcase{testcase.Name, "POST", "/restful/pubsub", meta, testcase.Httpcode, testcase.Resp}
	}

	t.Run("Get", func(t *testing.T) {

		for _, testcase := range []tc.GenericTestcase{

			// Only check error message when http status != 200
			// use nil as resp parameter for genericDoTest()
			tc.GenericTestcase{"FollowingPostOK", "GET", `/following/user?resource=post&id=71`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingPostReviewOK", "GET", `/following/user?resource=post&resource_type=review&id=71`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingPostNewsOK", "GET", `/following/user?resource=post&resource_type=news&id=71`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingProjectOK", "GET", `/following/user?resource=project&id=71`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingWithTargetIDsOK", "GET", `/following/user?resource=project&id=71&target_ids=[1,2,3]`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingWithModeIDOK", "GET", `/following/user?resource=project&id=71&mode=id`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingMultipleRes", "GET", `/following/user?resource=["post", "project"]&id=71`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingMaxresultPaging", "GET", `/following/user?resource=["post", "project"]&id=71&max_result=1&page=2`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingBadID", "GET", `/following/user?resource=post&max_result=1`, ``, http.StatusBadRequest, `{"Error":"Bad Resource ID"}`},
			tc.GenericTestcase{"FollowingBadType", "GET", `/following/user?resource=["post", "aaa"]&id=71`, ``, http.StatusBadRequest, `{"Error":"Bad Following Type"}`},

			tc.GenericTestcase{"FollowedPostOK", "GET", `/following/resource?resource=post&ids=[42,84]&resource_type=news`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowedPostReviewOK", "GET", `/following/resource?resource=post&ids=[42,84]&resource_type=review`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowedPostNewsOK", "GET", `/following/resource?resource=post&resource_type=news&ids=[42,84]`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowedMemberOK", "GET", `/following/resource?resource=member&ids=[42,84]`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowedProjectSingleOK", "GET", `/following/resource?resource=project&ids=[840]`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowedProjectOK", "GET", `/following/resource?resource=project&ids=[420,840]`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowedMissingResource", "GET", `/following/resource?ids=[420,840]`, ``, http.StatusBadRequest, `{"Error":"Unsupported Resource"}`},
			tc.GenericTestcase{"FollowedMissingID", "GET", `/following/resource?resource=post&ids=[]`, ``, http.StatusBadRequest, `{"Error":"Bad Resource ID"}`},
			tc.GenericTestcase{"FollowedPostNotExist", "GET", `/following/resource?resource=post&ids=[1000,1001]&resource_type=news`, ``, http.StatusOK, nil},
			tc.GenericTestcase{"FollowedPostStringID", "GET", `/following/resource?resource=post&ids=[unintegerable]`, ``, http.StatusBadRequest, `{"Error":"Bad Resource ID"}`},
			tc.GenericTestcase{"FollowedProjectStringID", "GET", `/following/resource?resource=project&ids=[unintegerable]`, ``, http.StatusBadRequest, `{"Error":"Bad Resource ID"}`},
			tc.GenericTestcase{"FollowedProjectInvalidEmotion", "GET", `/following/resource?resource=project&ids=[42,84]&resource_type=review&emotion=angry`, ``, http.StatusBadRequest, `{"Error":"Unsupported Emotion"}`},
		} {
			tc.GenericDoTest(testcase, t, nil)
		}
	})
	// It seems insert and delete shouldn't be tested here.
	t.Run("Insert", func(t *testing.T) {

		for _, testcase := range []tc.GenericTestcase{
			tc.GenericTestcase{"FollowingPostOK", "follow", `/restful/pubsub`, `{"resource":"post","subject":70,"object":84}`, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingMemberOK", "follow", `/restful/pubsub`, `{"resource":"member","subject":70,"object":72}`, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingProjectOK", "follow", `/restful/pubsub`, `{"resource":"project","subject":70,"object":840}`, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingTagOK", "follow", `/restful/pubsub`, `{"resource":"tag","subject":70,"object":1}`, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingMissingResource", "follow", `/restful/pubsub`, `{"resource":"","subject":70,"object":72}`, http.StatusOK, `{"Error":"Unsupported Resource"}`},
			tc.GenericTestcase{"FollowingMissingAction", "", `/restful/pubsub`, `{"resource":"post","subject":70,"object":72}`, http.StatusOK, `{"Error":"Bad Request"}`},
		} {
			tc.GenericDoTest(transformPubsub(testcase), t, nil)
		}
	})
	t.Run("Delete", func(t *testing.T) {

		for _, testcase := range []tc.GenericTestcase{
			tc.GenericTestcase{"FollowingPostOK", "unfollow", `/restful/pubsub`, `{"resource":"post","subject":70,"object":84}`, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingMemberOK", "unfollow", `/restful/pubsub`, `{"resource":"member","subject":70,"object":72}`, http.StatusOK, nil},
			tc.GenericTestcase{"FollowingProjectOK", "unfollow", `/restful/pubsub`, `{"resource":"project","subject":70,"object":840}`, http.StatusOK, nil},
		} {
			tc.GenericDoTest(transformPubsub(testcase), t, nil)
		}
	})
}
