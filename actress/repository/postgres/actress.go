package postgres

import (
	"context"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type actressRepository struct {
	orm         *ormKit.DB
	sqlcQueries *Queries
}

func CreateActressRepo(orm *ormKit.DB) (domain.ActressRepo, error) {
	originDB, err := orm.DB()
	if err != nil {
		return nil, errors.Wrap(err, "get origin db failed")
	}
	return &actressRepository{
		orm:         orm,
		sqlcQueries: &Queries{db: originDB},
	}, nil
}

func (a *actressRepository) SetActressPreview(id, previewURL string) error {
	// UPDATE actresses SET preview = preview WHERE id = id
	builder := sq.
		Update("actresses").
		Set("preview", previewURL).
		Where("id = ?", id)
	sql, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "to sql failed")
	}
	execResult := a.orm.Table("actresses").Exec(sql, args...)
	if err := execResult.Error; err != nil {
		return errors.Wrap(err, "exec failed")
	}
	if execResult.RowsAffected != 1 {
		return errors.New("delete rows affected count error, count: " + strconv.FormatInt(execResult.RowsAffected, 10))
	}
	return nil
}

func (a *actressRepository) AddActress(name, preview string) (string, error) {
	// INSERT INTO actresses (name,preview,created_at,updated_at) VALUES (name,preview,created_at,updated_at) RETURNING id;
	builder := sq.
		Insert("actresses").
		Columns("name", "preview", "created_at", "updated_at").
		Values(name, preview, time.Now(), time.Now()).
		Suffix(`RETURNING "id"`)
	sql, args, err := builder.ToSql()
	if err != nil {
		return "", errors.Wrap(err, "to sql failed")
	}
	var id string
	insertResult := a.orm.Table("actresses").Raw(sql, args...).Scan(&id)
	if err := insertResult.Error; err != nil {
		return "", errors.Wrap(err, "exec failed")
	}
	if insertResult.RowsAffected != 1 {
		return "", errors.New("insert rows affected count error, count: " + strconv.FormatInt(insertResult.RowsAffected, 10))
	}
	return id, nil
}

func (a *actressRepository) GetActress(id string) (*domain.Actress, error) {
	// SELECT * FROM actresses WHERE id = 3 LIMIT 1;
	builder := sq.
		Select("*").
		From("actresses").
		Where("id = ?", id).
		Limit(1)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var actress domain.Actress
	queryResult := a.orm.Table("actresses").Raw(sql, args...).Scan(&actress)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return &actress, nil
}

func (a *actressRepository) AddFavorite(userID, actressID string) error {
	// INSERT INTO account_favorite_actresses (id,actress_id,account_id,created_at) VALUES (id,info_id,account_id,created_at) RETURNING id;
	builder := sq.
		Insert("account_favorite_actresses").
		Columns("id", "actress_id", "account_id", "created_at").
		Values(utilKit.GetSnowflakeIDString(), actressID, userID, time.Now())
	sql, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "to sql failed")
	}
	execResult := a.orm.Table("account_favorite_actresses").Exec(sql, args...)
	err = execResult.Error
	if dbErr, ok := ormKit.ConvertDBLevelErr(err); ok && errors.Is(dbErr, ormKit.ErrDuplicatedKey) {
		return errors.Wrap(domain.ErrAlreadyDone, "already insert favorite. insert rows affected count error, count: "+strconv.FormatInt(execResult.RowsAffected, 10))
	} else if err != nil {
		return errors.Wrap(err, "exec failed")
	}
	if execResult.RowsAffected != 1 {
		return errors.New("insert rows affected count error, count: " + strconv.FormatInt(execResult.RowsAffected, 10))
	}
	return nil
}

func (a *actressRepository) GetFavorites(userID string) ([]*domain.Actress, error) {
	// SELECT * FROM actresses WHERE id IN (SELECT actress_id FROM account_favorite_actresses WHERE account_id = account_id);
	builder := sq.
		Select("*").
		From("actresses").
		Where("id IN (SELECT actress_id FROM account_favorite_actresses WHERE account_id = ?)", userID)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var actresses []*domain.Actress
	queryResult := a.orm.Table("actresses").Raw(sql, args...).Scan(&actresses)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return actresses, nil
}

func (a *actressRepository) RemoveFavorite(userID, actressID string) error {
	// DELETE FROM account_favorite_actresses WHERE actress_id = "123" AND account_id = "123";
	builder := sq.
		Delete("account_favorite_actresses").
		Where("actress_id = ? AND account_id = ?", actressID, userID)
	sql, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "to sql failed")
	}
	execResult := a.orm.Table("account_favorite_actresses").Exec(sql, args...)
	if err := execResult.Error; err != nil {
		return errors.Wrap(err, "exec failed")
	}
	if execResult.RowsAffected != 1 {
		return errors.New("delete rows affected count error, count: " + strconv.FormatInt(execResult.RowsAffected, 10))
	}
	return nil
}

func (a *actressRepository) GetWish() (*domain.Actress, error) {
	// SELECT * FROM actresses ORDER BY random() LIMIT 1;
	builder := sq.
		Select("*").
		From("actresses").
		OrderBy("random()"). // TODO: check performance
		Limit(1)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var actress domain.Actress
	queryResult := a.orm.Table("actresses").Raw(sql, args...).Scan(&actress)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return &actress, nil
}

func (a *actressRepository) GetActressByFaceToken(faceToken string) (*domain.Actress, error) {
	// SELECT * FROM actresses WHERE id = (SELECT actress_id FROM actress_faces WHERE token = '5507faaf83e00db98bd8a01a251b80dc')
	builder := sq.
		Select("*").
		From("actresses").
		Where("id = (SELECT actress_id FROM actress_faces WHERE token = ?)", faceToken)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var actress domain.Actress
	queryResult := a.orm.Table("actresses").Raw(sql, args...).Scan(&actress)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}

	if queryResult.RowsAffected < 1 {
		return nil, errors.Wrap(domain.ErrNoData, "not found data")
	}

	return &actress, nil
}

func (a *actressRepository) AddFace(actressID string, faceToken string, previewURL string) (faceID string, err error) {
	// INSERT INTO actress_faces (token,preview,actress_id,status,created_at,updated_at) VALUES (token,preview,actress_id,status,created_at,updated_at) RETURNING id;
	builder := sq.
		Insert("actress_faces").
		Columns("token", "preview", "actress_id", "status", "created_at", "updated_at").
		Values(faceToken, previewURL, actressID, domain.FaceStatusNotInFaceSet, time.Now(), time.Now()).
		Suffix(`RETURNING "id"`)
	sql, args, err := builder.ToSql()
	if err != nil {
		return "", errors.Wrap(err, "to sql failed")
	}
	var id string
	insertResult := a.orm.Table("actress_faces").Raw(sql, args...).Scan(&id)
	if err := insertResult.Error; err != nil {
		return "", errors.Wrap(err, "exec failed")
	}
	if insertResult.RowsAffected != 1 {
		return "", errors.New("insert rows affected count error, count: " + strconv.FormatInt(insertResult.RowsAffected, 10))
	}
	return id, nil
}

func (a *actressRepository) RemoveFace(faceID int) error {
	// DELETE FROM actress_faces WHERE id = 123;
	builder := sq.
		Delete("actress_faces").
		Where("id = ?", faceID)
	sql, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "to sql failed")
	}
	execResult := a.orm.Table("actress_faces").Exec(sql, args...)
	if err := execResult.Error; err != nil {
		return errors.Wrap(err, "exec failed")
	}
	if execResult.RowsAffected != 1 {
		return errors.New("delete rows affected count error, count: " + strconv.FormatInt(execResult.RowsAffected, 10))
	}
	return nil
}

func (a *actressRepository) GetActressByName(name string) (*domain.Actress, error) {
	// SELECT * FROM actresses WHERE name = 'ABC'
	builder := sq.
		Select("*").
		From("actresses").
		Where("name = ?", name)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var actress domain.Actress
	queryResult := a.orm.Table("actresses").Raw(sql, args...).Scan(&actress)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}

	if queryResult.RowsAffected < 1 {
		return nil, errors.Wrap(domain.ErrNoData, "not found data")
	}

	return &actress, nil
}

func (a *actressRepository) GetFacesByStatus(status domain.FaceStatus) ([]*domain.Face, error) {
	// SELECT * FROM actress_faces WHERE status = 123
	builder := sq.
		Select("*").
		From("actress_faces").
		Where("status = ?", status)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var faces []*domain.Face
	queryResult := a.orm.Table("actress_faces").Raw(sql, args...).Scan(&faces)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}

	if queryResult.RowsAffected < 1 {
		return nil, errors.Wrap(domain.ErrNoData, "not found data")
	}

	return faces, nil
}

func (a *actressRepository) SetFaceStatus(faceID int, faceSetToken string, status domain.FaceStatus) error {
	// UPDATE actress_faces SET token_set = 'abc' status = 2 WHERE id = 123;
	builder := sq.
		Update("actress_faces").
		Set("status", status).
		Set("token_set", faceSetToken).
		Where("id = ?", faceID)
	sql, args, err := builder.ToSql()
	if err != nil {
		return errors.Wrap(err, "to sql failed")
	}
	execResult := a.orm.Table("actress_faces").Exec(sql, args...)
	if err := execResult.Error; err != nil {
		return errors.Wrap(err, "exec failed")
	}
	if execResult.RowsAffected != 1 {
		return errors.New("delete rows affected count error, count: " + strconv.FormatInt(execResult.RowsAffected, 10))
	}
	return nil
}

func (a *actressRepository) GetFacesByActressID(actressID string) ([]*domain.Face, error) {
	// SELECT * FROM actress_faces WHERE actress_id = 123
	builder := sq.
		Select("*").
		From("actress_faces").
		Where("actress_id = ?", actressID)
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "to sql failed")
	}
	var faces []*domain.Face
	queryResult := a.orm.Table("actress_faces").Raw(sql, args...).Scan(&faces)
	if err := queryResult.Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}

	return faces, nil
}

func (a *actressRepository) GetActressesByPagination(ctx context.Context, page int, limit int) ([]*domain.Actress, int64, bool, error) {
	actressRows, err := a.sqlcQueries.GetActressesByPagination(ctx, GetActressesByPaginationParams{
		Offset: int32((page - 1) * limit),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, 0, false, errors.Wrap(err, "get actresses by page failed")
	}
	size, err := a.sqlcQueries.GetActressSize(ctx)
	if err != nil {
		return nil, 0, false, errors.Wrap(err, "get actresses size failed")
	}
	actresses := make([]*domain.Actress, len(actressRows))
	for idx, val := range actressRows {
		actresses[idx] = &domain.Actress{
			ID:           strconv.FormatInt(val.ID, 10),
			Name:         val.Name.String,
			Preview:      val.Preview.String,
			Detail:       val.Detail.String,
			Romanization: val.Romanization.String,
			CreatedAt:    val.CreatedAt.Time,
			UpdatedAt:    val.UpdatedAt.Time,
		}
	}
	var isEnd bool
	if int64((page-1)*limit+limit) >= size {
		isEnd = true
	}
	return actresses, size, isEnd, nil
}

func (a *actressRepository) GetActresses(ctx context.Context, ids ...int32) ([]*domain.Actress, error) {
	actressRows, err := a.sqlcQueries.GetActressesByIDs(ctx, ids)
	if err != nil {
		return nil, errors.Wrap(err, "get actresses by ids failed")
	}
	actresses := make([]*domain.Actress, len(actressRows))
	for idx, val := range actressRows {
		actresses[idx] = &domain.Actress{
			ID:           strconv.FormatInt(val.ID, 10),
			Name:         val.Name.String,
			Preview:      val.Preview.String,
			Detail:       val.Detail.String,
			Romanization: val.Romanization.String,
			CreatedAt:    val.CreatedAt.Time,
			UpdatedAt:    val.UpdatedAt.Time,
		}
	}
	return actresses, nil
}
