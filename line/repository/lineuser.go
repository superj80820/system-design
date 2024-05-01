package repository

import (
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type lineUserRepo struct {
	orm *ormKit.DB
}

func CreateLineUserRepo(orm *ormKit.DB) domain.LineUserRepo {
	return &lineUserRepo{
		orm: orm,
	}
}

func (l *lineUserRepo) Create(userID int64, lineUserID string, displayName string, pictureURL string) (*domain.LineUserProfile, error) {
	// INSERT INTO line_profile (id,user_id,line_id,display_name,picture_url,created_at,updated_at) VALUES (id,user_id,line_id,display_name,picture_url,createdat,updatedat);
	builder := sq.
		Insert("line_profile").
		Columns("id", "user_id", "line_id", "display_name", "picture_url", "created_at", "updated_at").
		Values(utilKit.GetUUIDString(), userID, lineUserID, displayName, pictureURL, time.Now(), time.Now()) // TODO: WORKAROUND: change id to snowflake form uuid
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	insertResult := l.orm.Table("line_profile").Exec(sql, args...)
	if err := insertResult.Error; err != nil {
		return nil, errors.Wrap(err, "exec failed")
	}
	if insertResult.RowsAffected != 1 {
		return nil, errors.New("insert rows affected count error, count: " + strconv.FormatInt(insertResult.RowsAffected, 10))
	}

	return &domain.LineUserProfile{
		UserID:      userID,
		LineUserID:  lineUserID,
		DisplayName: displayName,
		PictureURL:  pictureURL,
	}, nil
}

func (l *lineUserRepo) Get(lineUserID string) (*domain.LineUserProfile, error) {
	// SELECT * FROM line_profile WHERE line_id = 'abcd' LIMIT 1
	builder := sq.
		Select("*").
		From("line_profile").
		Where("line_id = ?", lineUserID).
		Limit(1)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var lineUserProfile domain.LineUserProfile
	queryResult := l.orm.Table("line_profile").Raw(sql, args...).Scan(&lineUserProfile)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}

	if queryResult.RowsAffected < 1 {
		return nil, errors.Wrap(domain.ErrNoData, "not found data")
	}

	return &lineUserProfile, nil
}
