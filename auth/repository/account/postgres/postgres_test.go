package postgres

import (
	"context"
	"log"
	"testing"
	"time"

	"database/sql"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestActressRepo(t *testing.T) {
	ctx := context.Background()
	// orm, err := ormKit.CreateDB(ormKit.UsePostgres()
	// assert.Nil(t, err)

	db, err := sql.Open("postgres", "user=postgres password=postgres dbname=messfar sslmode=disable port=5433")
	assert.Nil(t, err)

	repo := New(db)

	all, err := repo.GetAll(ctx)
	assert.Nil(t, err)

	allIDs := make([]string, len(all))
	for idx, val := range all {
		allIDs[idx] = val.ID
	}

	for _, val := range allIDs {
		log.Println("-----1. update id: " + val)

		timeNow := time.Now()

		err := repo.UpdateUpdatedTime(ctx, UpdateUpdatedTimeParams{
			UpdatedAt: sql.NullTime{
				Time:  timeNow,
				Valid: true,
			},
			ID: val,
		})
		assert.Nil(t, err)

		log.Println("-----2. update time: " + timeNow.String())
	}
}
