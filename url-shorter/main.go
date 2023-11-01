package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jxskiss/base62"
	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/bwmarrin/snowflake"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
)

type DB map[int64]*URLEntity

var singletonDB *gorm.DB

type URLEntity struct {
	ID        int64
	ShortURL  string
	LongURL   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (URLEntity) TableName() string {
	return "url"
}

func GetSingletonDB() *gorm.DB { // TODO: york
	if singletonDB != nil {
		return singletonDB
	}
	dsn := "root:example@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local" // TODO: york
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln(err) // TODO: york
	}
	singletonDB = db
	return singletonDB
}

func (d DB) Save(urlEntity *URLEntity) {
	d[urlEntity.ID] = urlEntity
}

func (d DB) Get(id int64) (*URLEntity, bool) {
	if val, ok := d[id]; ok {
		return val, true
	}
	return nil, false
}

type URLService interface {
	Save(url string) (string, error)
	Get(shortURL string) (string, bool, error)
}

type urlService struct{}

func checkPrefixURL(url, prefix string) bool {
	for i := 0; i < len(prefix); i++ {
		if url[i] != prefix[i] {
			return false
		}
	}
	return true
}

func (u urlService) Save(url string) (string, error) {
	if !checkPrefixURL(url, "https://") && !checkPrefixURL(url, "http://") {
		return "", errors.New("error url format")
	}

	uniqueIDGenerate, err := GetUniqueIDGenerate()
	if err != nil {
		// TODO: york
		log.Fatalln(err)
	}

	uniqueID := uniqueIDGenerate.Generate()
	shortURLID := uniqueID.GetInt64()
	shortURL := uniqueID.GetBase62()

	// TODO: cache

	// TODO: save to real DB
	err = GetSingletonDB().Create(&URLEntity{
		ID:       shortURLID,
		LongURL:  url,
		ShortURL: shortURL,
	}).Error
	if err != nil {
		return "", errors.Wrap(err, "save url data failed")
	}

	return shortURL, nil
}

func (u urlService) Get(shortURL string) (string, bool, error) {
	shortURLID, err := base62.ParseInt([]byte(shortURL)) // TODO: york
	if err != nil {
		return "", false, errors.Wrap(err, "parse short url to number failed")
	}

	// TODO: cache

	// TODO: get by real DB
	var url URLEntity
	if GetSingletonDB().First(&url, shortURLID).Error != nil {
		return "", false, nil
	}
	return url.LongURL, true, nil
}

type urlShortenRequest struct {
	LongURL string `json:"longURL"`
}
type urlShortenResponse struct {
	Data  *urlShortenResponseData `json:"data,omitempty"`
	Error *ResponseError          `json:"error,omitempty"`
}

type urlShortenResponseData struct {
	ShortURL string `json:"shortURL"`
}

func makeURLShortenEndpoint(svc URLService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(urlShortenRequest)
		shortURL, err := svc.Save(req.LongURL)
		if err != nil {
			return urlShortenResponse{
				Error: &ResponseError{Message: err.Error()}, // TODO: york
			}, nil
		}
		return urlShortenResponse{Data: &urlShortenResponseData{
			ShortURL: shortURL,
		}}, nil
	}
}

type urlGetRequest struct {
	ShortURL string `json:"shortURL"`
}
type urlGetResponse struct {
	Data  *urlGetResponseData `json:"data,omitempty"`
	Error *ResponseError      `json:"error,omitempty"`
}

type urlGetResponseData struct {
	LongURL string `json:"longURL"`
}

type ResponseError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func makeURLGetEndpoint(svc URLService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(urlGetRequest)
		longURL, exist, err := svc.Get(req.ShortURL)
		if err != nil {
			return urlGetResponse{
				Error: &ResponseError{Message: "internal error"},
			}, nil
		}
		if !exist {
			return urlGetResponse{
				Error: &ResponseError{Message: "not found url"},
			}, nil
		}
		return urlGetResponse{Data: &urlGetResponseData{
			LongURL: longURL,
		}}, nil
	}
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

type UniqueIDGenerate struct {
	snowflakeNode *snowflake.Node
}

var singletonUniqueIDGenerate *UniqueIDGenerate

func GetUniqueIDGenerate() (*UniqueIDGenerate, error) {
	if singletonUniqueIDGenerate != nil {
		return singletonUniqueIDGenerate, nil // TODO: york
	}
	snowflakeNode, err := snowflake.NewNode(1) // TODO: york
	if err != nil {
		return nil, errors.Wrap(err, "create snowflake failed")
	}
	singletonUniqueIDGenerate = &UniqueIDGenerate{
		snowflakeNode: snowflakeNode,
	}
	return singletonUniqueIDGenerate, nil
}

func (u UniqueIDGenerate) Generate() *UniqueID {
	return &UniqueID{
		snowflakeID: u.snowflakeNode.Generate(),
	}
}

type UniqueID struct {
	snowflakeID snowflake.ID
}

func (u UniqueID) GetInt64() int64 {
	return u.snowflakeID.Int64() // TODO: york unit test
}

func (u UniqueID) GetBase62() string {
	return string(base62.FormatInt(u.snowflakeID.Int64())) // TODO: york unit test
}

func powInt(x, y int) int64 {
	return int64(math.Pow(float64(x), float64(y)))
}

func main() {
	r := mux.NewRouter()

	var urlSVC urlService
	urlShortenHandler := httptransport.NewServer(
		makeURLShortenEndpoint(urlSVC),
		decodeURLShortenRequests,
		encodeURLShortenResponse,
		httptransport.ServerErrorEncoder(encodeError),
	)
	urlGetHandler := httptransport.NewServer(
		makeURLGetEndpoint(urlSVC),
		decodeURLGetRequests,
		encodeURLGetResponse,
		httptransport.ServerErrorEncoder(encodeError),
	)

	r.Methods("POST").Path("/api/v1/data/shorten").Handler(urlShortenHandler)
	r.Methods("GET").Path("/api/v1/shortUrl").Handler(urlGetHandler)
	log.Fatal(http.ListenAndServe(":9090", r))
}

func decodeURLShortenRequests(ctx context.Context, r *http.Request) (interface{}, error) { // TODO: york can use interface for r
	var request urlShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, errors.New("decode body error: " + err.Error())
	}
	return request, nil
}

func encodeURLShortenResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

func decodeURLGetRequests(ctx context.Context, r *http.Request) (interface{}, error) {
	var request urlGetRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, errors.New("decode body error: " + err.Error())
	}
	return request, nil
}

func encodeURLGetResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}
