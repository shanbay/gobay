package gormext

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"sort"
	"strings"
	"time"
)

// Updates are values for `ON CONFLICT UPDATE`
type Updates map[string]interface{}

const (
	IGNORE = "IGNORE"
)

// Model base model definition, including fields `ID`, `CreatedAt`, `UpdatedAt`, which could be embedded in your models
//    type User struct {
//      gorm.Model
//    }
type Model struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Register Callbacks
func registerCallbacks(db *gorm.DB) {
	db.Callback().Create().Before("gorm:create").Register("gormext:before_create_on_conflict", onConflictCallback)
}

// onConflictCallback: Ignore and ON CONFLICT
func onConflictCallback(scope *gorm.Scope) {
	obj, ok := scope.Get("gormext:on_conflict")
	if !ok {
		return
	}
	switch obj.(type) {
	default:
		// ON CONFLICT KEY UPDATE
		newScope := scope.NewDB().NewScope(obj)
		columns := []string{}
		updateMap := map[string]interface{}{}
		for _, field := range newScope.Fields() {
			if !field.IsBlank {
				columns = append(columns, field.DBName)
				updateMap[field.DBName] = field.Field.Interface()
			}
		}
		sort.Strings(columns)
		sqls := []string{}
		for _, column := range columns {
			value := updateMap[column]
			sqls = append(sqls, fmt.Sprintf("%v = %v", newScope.Quote(column), newScope.AddToVars(value)))
		}
		sql := strings.Join(sqls, ",")
		scope.Set("gorm:insert_option", fmt.Sprintf("ON CONFLICT KEY UPDATE %v", sql))
	case string:
		// INSERT IGNORE INTO
		scope.Set("gorm:insert_modifier", obj)
	}
}

// InsertMany

// GetOrCreate
