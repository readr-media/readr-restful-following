package model

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/readr-media/readr-restful-following/config"
	"github.com/readr-media/readr-restful-following/internal/rrsql"
)

type Resource struct {
	ResourceName string `form:"resource" json:"resource"`
	ResourceType string `form:"resource_type" json:"resource_type, omitempty"`
	Table        string
	PrimaryKey   string
	FollowType   int
	Emotion      int
	MaxResult    int `form:"max_result"`
	Page         int `form:"page"`
}

type FollowingSQL struct {
	base      string
	condition []string
	args      []interface{}
	printargs []interface{}
	join      []string
	postfixes []string
}

func (f *FollowingSQL) AppendPrintarg(arg interface{}) {
	f.printargs = append(f.printargs, arg)
}

func (f *FollowingSQL) PrependPrintarg(arg interface{}) {
	f.printargs = append([]interface{}{arg}, f.printargs...)
}

func (f *FollowingSQL) AppendArg(arg interface{}) {
	f.args = append(f.args, arg)
}

func (f *FollowingSQL) AppendCondition(arg string) {
	f.condition = append(f.condition, arg)
}

func (f *FollowingSQL) SQL() string {
	return fmt.Sprintf(f.base, f.printargs...)
}

type FollowArgs struct {
	Resource string
	Subject  int64
	Object   int64
	Type     int
	Emotion  int
}

/* ================================================ Get Following ================================================ */

type FollowingItem struct {
	Type       int            `db:"type" json:"type"`
	TargetID   int            `db:"target_id" json:"target_id"`
	FollowedAt rrsql.NullTime `db:"created_at" json:"followed_at"`
}

type GetFollowInterface interface {
	get() (*sqlx.Rows, error)
	scan(*sqlx.Rows) (interface{}, error)
}

type GetFollowingArgs struct {
	MemberID  int64  `form:"id" json:"id"`
	Mode      string `form:"mode"`
	MaxResult int    `form:"max_result"`
	Page      int    `form:"page"`
	TargetIDs []int
	Active    map[string][]int
	Resource
	Resources []string
}

func (g *GetFollowingArgs) get() (*sqlx.Rows, error) {
	// change resource name to int type
	followType := make([]int, 0)
	for _, resourceName := range g.Resources {
		ft, err := g.getFollowType(resourceName)
		if err != nil {
			return nil, err
		}
		followType = append(followType, ft)
	}

	var osql = FollowingSQL{
		base: `SELECT f.type, f.target_id, f.created_at FROM following AS f %s 
		WHERE %s ORDER BY f.created_at DESC %s;`,
		printargs: []interface{}{},
		condition: []string{"f.type IN (?)", "f.member_id = ?", "f.emotion = ?"},
		args:      []interface{}{followType, g.MemberID, 0},
	}
	// Append post's type filter to printarg
	if g.ResourceType != "" {
		if val, ok := config.Config.Models.PostType[g.ResourceType]; ok {
			osql.AppendPrintarg(` LEFT JOIN posts AS p ON f.target_id = p.post_id `)
			osql.AppendCondition(fmt.Sprintf(" NOT (p.type <> ? AND f.type = %d)", config.Config.Models.FollowingType["post"]))
			osql.AppendArg(val)
		} else if g.ResourceType != "" {
			return nil, errors.New("Invalid Post Type")
		}
	} else {
		osql.AppendPrintarg("")
	}

	if len(g.TargetIDs) > 0 {
		osql.AppendCondition("f.target_id IN (?)")
		osql.AppendArg(g.TargetIDs)
	}

	osql.AppendPrintarg(strings.Join(osql.condition, " AND "))

	if g.MaxResult != 0 {
		if g.Page != 0 {
			osql.AppendPrintarg(" LIMIT ? OFFSET ? ")
			osql.AppendArg(g.MaxResult)
			osql.AppendArg((g.Page - 1) * g.MaxResult)
		} else {
			osql.AppendPrintarg(" LIMIT ? ")
			osql.AppendArg(g.MaxResult)
		}
	} else {
		osql.AppendPrintarg("")
	}
	query, args, err := sqlx.In(osql.SQL(), osql.args...)
	query = rrsql.DB.Rebind(query)

	rows, err := rrsql.DB.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	return rows, err
}

func (g *GetFollowingArgs) scan(rows *sqlx.Rows) (result interface{}, err error) {

	if g.Mode == "id" {
		var followingIDs []int
		for rows.Next() {
			var f FollowingItem
			err = rows.StructScan(&f)
			if err != nil {
				log.Println(fmt.Sprintf("Fail Scan Following Items: %v", err.Error()))
			}
			followingIDs = append(followingIDs, f.TargetID)
		}
		return followingIDs, nil
	}

	followingResults := make([]FollowingItem, 0)
	for rows.Next() {
		var f FollowingItem
		err := rows.StructScan(&f)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Scan following item error: %s", err.Error()))
		}
		followingResults = append(followingResults, f)
	}

	return followingResults, err
}

func (g *GetFollowingArgs) getFollowType(resourceName string) (t int, err error) {
	if val, ok := config.Config.Models.FollowingType[resourceName]; ok {
		return val, nil
	}
	return t, errors.New("Unsupported Following Type")
}

/* ================================================ Get Followed ================================================ */

type GetFollowedArgs struct {
	IDs []int64 `json:"ids"`
	Resource
}

type FollowedCount struct {
	ResourceID int64   `json:"ResourceID"`
	Count      int     `json:"Count"`
	Followers  []int64 `json:"Followers"`
}

func (g *GetFollowedArgs) get() (*sqlx.Rows, error) {

	var osql = FollowingSQL{
		base: `SELECT f.target_id, COUNT(m.id) as count, 
		GROUP_CONCAT(m.id SEPARATOR ',') as follower FROM following as f 
		LEFT JOIN %s WHERE %s GROUP BY f.target_id;`,
		condition: []string{"f.target_id IN (?)", "f.type = ?", "f.emotion = ?"},
		join:      []string{"members AS m ON f.member_id = m.id"},
		args:      []interface{}{g.IDs, g.FollowType, g.Emotion},
	}
	query, args, err := sqlx.In(fmt.Sprintf(osql.base, strings.Join(osql.join, " LEFT JOIN "), strings.Join(osql.condition, " AND ")), osql.args...)
	if err != nil {
		return nil, err
	}
	query = rrsql.DB.Rebind(query)
	return rrsql.DB.Queryx(query, args...)
}

func (g *GetFollowedArgs) scan(rows *sqlx.Rows) (interface{}, error) {

	var (
		followed []FollowedCount
		err      error
	)
	for rows.Next() {
		var (
			resourceID int64
			count      int
			follower   string
		)
		err = rows.Scan(&resourceID, &count, &follower)
		if err != nil {
			log.Fatalln(err.Error())
			return nil, err
		}
		var followers []int64
		for _, v := range strings.Split(follower, ",") {
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			followers = append(followers, int64(i))
		}
		followed = append(followed, FollowedCount{resourceID, count, followers})
	}
	return followed, err
}

/* ================================================ Get Follow Map ================================================ */

type GetFollowMapArgs struct {
	UpdateAfter time.Time `form:"updated_after" json:"updated_after"`
	Resource
}

func (g *GetFollowMapArgs) get() (*sqlx.Rows, error) {
	var osql = FollowingSQL{
		base: `SELECT GROUP_CONCAT(member_resource.member_id) AS member_ids, member_resource.resource_ids
			FROM (
				SELECT GROUP_CONCAT(f.target_id) AS resource_ids, m.id AS member_id 
				FROM following AS f
				LEFT JOIN %s
				WHERE %s
				GROUP BY m.id
				) AS member_resource
			GROUP BY member_resource.resource_ids;`,
		join:      []string{"members AS m ON f.member_id = m.id", fmt.Sprintf("%s AS t ON f.target_id = t.%s", g.Table, g.PrimaryKey)},
		condition: []string{"m.active = ?", "m.post_push = ?", "f.type = ?"},
		args:      []interface{}{config.Config.Models.Members["active"], 1, g.FollowType},
	}

	switch g.ResourceName {
	case "member":
		osql.join = append(osql.join, "posts AS p ON f.target_id = p.author")
		osql.condition = append(osql.condition, "t.active = ?", "p.active = ?", "p.publish_status = ?", "p.updated_at > ?")
		osql.args = append(osql.args, config.Config.Models.Members["active"], config.Config.Models.Posts["active"], config.Config.Models.PostPublishStatus["publish"], g.UpdateAfter)
	case "post":
		osql.condition = append(osql.condition, "t.active = ?", "t.publish_status = ?", "t.updated_at > ?")
		osql.args = append(osql.args, config.Config.Models.Posts["active"], config.Config.Models.PostPublishStatus["publish"], g.UpdateAfter)
	case "project":
		osql.condition = append(osql.condition, "t.active = ?", "t.publish_status = ?", "t.updated_at > ?")
		osql.args = append(osql.args, config.Config.Models.ProjectsActive["active"], config.Config.Models.ProjectsPublishStatus["publish"], g.UpdateAfter)
	}

	rows, err := rrsql.DB.Queryx(fmt.Sprintf(osql.base, strings.Join(osql.join, " LEFT JOIN "), strings.Join(osql.condition, " AND ")), osql.args...)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	return rows, err
}

func (g *GetFollowMapArgs) scan(rows *sqlx.Rows) (interface{}, error) {

	var (
		list []FollowingMapItem
		err  error
	)
	for rows.Next() {
		var memberIDs, resourceIDs string
		if err = rows.Scan(&memberIDs, &resourceIDs); err != nil {
			log.Println(err)
			return []FollowingMapItem{}, err
		}
		list = append(list, FollowingMapItem{
			strings.Split(memberIDs, ","),
			strings.Split(resourceIDs, ","),
		})
	}
	return list, err
}

type GetFollowerMemberIDsArgs struct {
	ID         int64
	FollowType int
	Emotions   []int
}

type FollowingMapItem struct {
	Followers   []string `json:"member_ids" db:"member_ids"`
	ResourceIDs []string `json:"resource_ids" db:"resource_ids"`
}

func (g *GetFollowerMemberIDsArgs) get() (*sqlx.Rows, error) {

	query, args, err := sqlx.In(`SELECT member_id FROM following WHERE target_id = ? AND type = ? AND emotion IN (?);`, g.ID, g.FollowType, g.Emotions)
	query = rrsql.DB.Rebind(query)

	rows, err := rrsql.DB.Queryx(query, args...)
	if err != nil {
		log.Printf("Error: %v get Follower for id:%d, type:%d\n", err.Error(), g.ID, g.FollowType)
	}
	return rows, err

}

func (g *GetFollowerMemberIDsArgs) scan(rows *sqlx.Rows) (interface{}, error) {
	var (
		result []int
		err    error
	)
	for rows.Next() {
		var follower int
		err = rows.Scan(&follower)
		if err != nil {
			log.Printf("Error: %v scan for id:%d, type:%d\n", err.Error(), g.ID, g.FollowType)
			return nil, err
		}
		result = append(result, follower)
	}
	return result, err
}

/* ================================================ Following API ================================================ */

type followingAPI struct{}

type FollowingAPIInterface interface {
	Get(params GetFollowInterface) (interface{}, error)
	Insert(params FollowArgs) error
	Update(params FollowArgs) error
	Delete(params FollowArgs) error
}

func (f *followingAPI) Get(params GetFollowInterface) (result interface{}, err error) {

	var rows *sqlx.Rows

	rows, err = params.get()
	if err != nil {
		log.Println("Error Get Follow with params.get()")
		return nil, err
	}
	return params.scan(rows)
}

func (f *followingAPI) Insert(params FollowArgs) (err error) {

	query := `INSERT INTO following (member_id, target_id, type, emotion) VALUES ( ?, ?, ?, ?);`

	result, err := rrsql.DB.Exec(query, params.Subject, params.Object, params.Type, params.Emotion)
	if err != nil {
		sqlerr, ok := err.(*mysql.MySQLError)
		if ok && sqlerr.Number == 1062 {
			return rrsql.DuplicateError
		}
		log.Println(err.Error())
		return rrsql.InternalServerError
	}
	changed, err := result.RowsAffected()
	if err != nil {
		log.Println(err.Error())
		return rrsql.InternalServerError
	}
	if changed == 0 {
		return rrsql.SQLInsertionFail
	}

	return nil
}

func (f *followingAPI) Update(params FollowArgs) (err error) {

	result, err := rrsql.DB.Exec(`UPDATE following SET emotion = ? WHERE member_id = ? AND target_id = ? AND type = ? AND emotion != 0;`, params.Emotion, params.Subject, params.Object, params.Type)
	if err != nil {
		log.Println(err.Error())
		return rrsql.InternalServerError
	}
	changed, err := result.RowsAffected()
	if err != nil {
		log.Println(err.Error())
		return rrsql.InternalServerError
	}
	if changed == 0 {
		return rrsql.SQLUpdateFail
	}
	return nil
}

func (f *followingAPI) Delete(params FollowArgs) (err error) {
	query := `DELETE FROM following WHERE member_id = ? AND target_id = ? AND type = ? AND emotion = ?;`
	_, err = rrsql.DB.Exec(query, params.Subject, params.Object, params.Type, params.Emotion)
	if err != nil {
		log.Fatal(err)
	}

	return err
}

var FollowingAPI FollowingAPIInterface = new(followingAPI)
