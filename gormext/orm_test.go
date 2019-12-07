package gormext

import (
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shanbay/gobay"
)

var (
	age0     uint16 = 0
	age5     uint16 = 5
	username string = "whois that"
	db       *gorm.DB
)

func init() {
	bapp, err := gobay.CreateApp(
		"../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"dbext": &GormExt{},
		},
	)
	if err != nil || bapp == nil {
		panic(err)
	}
	db = bapp.Get("dbext").Object().(*gorm.DB)
}

type User struct {
	Model
	Username string
	Age      *uint16
}

func Test_onConflictCallback(t *testing.T) {
	type args struct {
		user *User
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "age0",
			args: args{
				user: &User{
					Age: &age0,
				},
			},
			want: "ON CONFLICT KEY UPDATE \"age\" = $$$",
		},
		{
			name: "age5",
			args: args{
				user: &User{
					Age: &age5,
				},
			},
			want: "ON CONFLICT KEY UPDATE \"age\" = $$$",
		},
		{
			name: "username_and_age0",
			args: args{
				user: &User{
					Age:      &age0,
					Username: username,
				},
			},
			want: "ON CONFLICT KEY UPDATE \"age\" = $$$,\"username\" = $$$",
		},
		{
			name: "username_and_age5",
			args: args{
				user: &User{
					Age:      &age5,
					Username: username,
				},
			},
			want: "ON CONFLICT KEY UPDATE \"age\" = $$$,\"username\" = $$$",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := db.NewScope(&User{})

			scope.Set("gormext:on_conflict", tt.args.user)
			onConflictCallback(scope)
			if result, ok := scope.Get("gorm:insert_option"); !ok {
				t.Errorf("insert_option should return: %v", result)
			} else if result.(string) != tt.want {
				t.Errorf("want %s, got %s", tt.want, result)
			}
		})
	}
}
