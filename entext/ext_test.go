package entext

import (
	"context"
	"github.com/facebookincubator/ent/dialect"
	_ "github.com/go-sql-driver/mysql"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/testdata/ent"
	"github.com/shanbay/gobay/testdata/ent/user"
	"testing"
)

var entclient *ent.Client

func setup() func() error {
	exts := map[gobay.Key]gobay.Extension{
		"entext": &EntExt{
			NS: "db_",
			NewClient: func(drvopt interface{}) Client {
				return ent.NewClient(drvopt.(ent.Option))
			},
			Driver: func(drv dialect.Driver) interface{} {
				return ent.Driver(drv)
			},
		},
	}
	app, err := gobay.CreateApp("../testdata", "testing", exts)
	if err != nil {
		panic(err)
	}
	entclient = app.Get("entext").Object().(*ent.Client)
	return app.Close
}

func TestEnt(t *testing.T) {
	defer setup()()
	ctx := context.Background()
	// migrate
	if err := entclient.Schema.Create(ctx); err != nil {
		t.Errorf("migration error: %v", err)
	}
	// create
	jeff, err := entclient.User.Create().SetUsername("jeff").Save(ctx)
	if err != nil {
		t.Errorf("create failed: %v", err)
	}
	if jeff.Username != "jeff" || jeff.Nickname != "jeff" {
		t.Errorf("username or nickname not jeff")
	}
	// get by id
	if entclient.User.Query().Where(user.ID(jeff.ID)).FirstX(ctx).Nickname != "jeff" {
		t.Errorf("user nickname not jeff")
	}
	// update
	entclient.User.Update().SetNickname("bob").Save(ctx)
	if entclient.User.Query().Where(user.ID(jeff.ID)).FirstX(ctx).Nickname != "bob" {
		t.Errorf("user nickname not bob")
	}
	// delete
	entclient.User.DeleteOneID(jeff.ID).Exec(ctx)
	if _, err := entclient.User.Query().Where(
		user.ID(jeff.ID),
	).First(ctx); err == nil || !ent.IsNotFound(err) {
		t.Errorf("expected Err NotFound, got %v", err)
	}
}
