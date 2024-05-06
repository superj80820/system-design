package http

import (
	"context"
	"io"
	"net/http"
	"net/textproto"
	"strconv"

	"github.com/go-kit/kit/endpoint"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
	httpKit "github.com/superj80820/system-design/kit/http"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type getActressRequest struct {
	ActressID string `json:"actress_id"`
}

type uploadSearchImageRequest struct {
	ImageFile *searchImage
}

type searchImage struct {
	rawData []byte
	name    string
	size    int64
	mime    textproto.MIMEHeader
}

type addFavoriteRequest struct {
	ActressID string `json:"actress_id"`
}

type searchActressByNameRequest struct {
	Page        uint   `json:"page"`
	Limit       uint   `json:"limit"`
	ActressName string `json:"actress_name"`
}

type getFavoritesRequest struct {
	Page        uint   `json:"page"`
	Limit       uint   `json:"limit"`
	ActressName string `json:"actress_name"`
}

type removeFavoriteRequest struct {
	ActressID string `json:"actress_id"`
}

var (
	EncodeSearchActressByNameResponse = httpTransportKit.EncodeJsonResponse

	EncodeGetActressResponse = httpTransportKit.EncodeJsonResponse

	EncodeUploadSearchImageResponse = httpTransportKit.EncodeJsonResponse

	EncodeGetFavoritesResponse = httpTransportKit.EncodeJsonResponse

	DecodeAddFavoriteRequest   = httpTransportKit.DecodeJsonRequest[addFavoriteRequest]
	EncodeAddFavoritesResponse = httpTransportKit.EncodeOKResponse

	EncodeRemoveFavoritesResponse = httpTransportKit.EncodeOKResponse
)

func MakeGetActressEndpoint(actressUseCase domain.ActressUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(getActressRequest)
		actress, err := actressUseCase.GetActress(req.ActressID)
		if err != nil {
			return nil, errors.Wrap(err, "get actress failed")
		}
		return actress, nil
	}
}

func MakeSearchActressesByNameEndpoint(actressUseCase domain.ActressUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(searchActressByNameRequest)
		actresses, err := actressUseCase.SearchActressByNamePagination(ctx, req.ActressName, req.Page, req.Limit)
		if err != nil {
			return nil, errors.Wrap(err, "get favorites failed")
		}
		return actresses, nil
	}
}

func MakeGetFavoritesEndpoint(actressUseCase domain.ActressUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id")
		}
		req := request.(getFavoritesRequest)
		favorites, err := actressUseCase.GetFavoritesPagination(ctx, strconv.Itoa(userID), req.Page, req.Limit)
		if err != nil {
			return nil, errors.Wrap(err, "get favorites failed")
		}
		return favorites, nil
	}
}

func MakeAddFavoriteEndpoint(actressUseCase domain.ActressUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id")
		}

		req := request.(addFavoriteRequest)
		if err := actressUseCase.AddFavorite(strconv.Itoa(userID), req.ActressID); err != nil {
			return nil, errors.Wrap(err, "add favorites failed")
		}
		return nil, nil
	}
}

func MakeUploadSearchImageEndpoint(actressUseCase domain.ActressUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(uploadSearchImageRequest)
		actresses, err := actressUseCase.SearchActressByFace(req.ImageFile.rawData)
		if err != nil {
			return nil, errors.Wrap(err, "search actress by face failed")
		}
		return actresses, nil
	}
}

func MakeRemoveFavoriteEndpoint(actressUseCase domain.ActressUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return nil, errors.New("not found user id")
		}

		req := request.(removeFavoriteRequest)
		if err := actressUseCase.RemoveFavorite(strconv.Itoa(userID), req.ActressID); err != nil {
			return nil, errors.Wrap(err, "remove favorites failed")
		}
		return nil, nil
	}
}

func DecodeGetActressRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	actressID, ok := vars["actressID"]
	if !ok {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get actress id failed"))
	}
	return getActressRequest{ActressID: actressID}, nil
}

func DecodeSearchActressByNameRequests(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	actressName, ok := vars["actressName"]
	if !ok {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get actress id failed"))
	}
	var page uint = 1
	pageQueryString := r.URL.Query().Get("page")
	if pageQueryString != "" {
		pageQueryUINT64, err := strconv.ParseUint(pageQueryString, 10, 64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("page format error"))
		}
		pageQueryUINT, err := utilKit.SafeUint64ToUint(pageQueryUINT64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("page to big error"))
		}
		page = pageQueryUINT
	}
	var limit uint = 10
	limitQueryString := r.URL.Query().Get("limit")
	if limitQueryString != "" {
		limitQueryUINT64, err := strconv.ParseUint(limitQueryString, 10, 64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("limit format error"))
		}
		limitQueryUINT, err := utilKit.SafeUint64ToUint(limitQueryUINT64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("limit to big error"))
		}
		limit = limitQueryUINT
	}
	return searchActressByNameRequest{ActressName: actressName, Page: page, Limit: limit}, nil
}

func DecodeRemoveFavoriteRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	actressID, ok := vars["actressID"]
	if !ok {
		return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("get actress id failed"))
	}
	return removeFavoriteRequest{ActressID: actressID}, nil
}

func DecodeGetFavoritesRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var page uint = 1
	pageQueryString := r.URL.Query().Get("page")
	if pageQueryString != "" {
		pageQueryUINT64, err := strconv.ParseUint(pageQueryString, 10, 64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("page format error"))
		}
		pageQueryUINT, err := utilKit.SafeUint64ToUint(pageQueryUINT64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("page to big error"))
		}
		page = pageQueryUINT
	}
	var limit uint = 10
	limitQueryString := r.URL.Query().Get("limit")
	if limitQueryString != "" {
		limitQueryUINT64, err := strconv.ParseUint(limitQueryString, 10, 64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("limit format error"))
		}
		limitQueryUINT, err := utilKit.SafeUint64ToUint(limitQueryUINT64)
		if err != nil {
			return nil, code.CreateErrorCode(http.StatusBadRequest).AddErrorMetaData(errors.New("limit to big error"))
		}
		limit = limitQueryUINT
	}
	return getFavoritesRequest{Page: page, Limit: limit}, nil
}

func DecodeUploadSearchImageRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	err := r.ParseMultipartForm(20 << 20) // 20 MB
	if err != nil {
		return nil, errors.Wrap(err, "too big image")
	}

	file, header, err := r.FormFile("image_file")
	if err != nil {
		return nil, errors.Wrap(err, "get image file failed")
	}
	defer file.Close()

	imageFile, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, "read image file failed")
	}

	return uploadSearchImageRequest{
		ImageFile: &searchImage{
			rawData: imageFile,
			name:    header.Filename,
			size:    header.Size,
			mime:    header.Header,
		},
	}, nil
}
