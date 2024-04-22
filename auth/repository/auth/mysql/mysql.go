package mysql

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type accountTokenEntity struct {
	*domain.AccountToken
}

func (accountTokenEntity) TableName() string {
	return "account_token"
}

type authRepo struct {
	db              *ormKit.DB
	accessTokenKey  *ecdsa.PrivateKey
	refreshTokenKey *ecdsa.PrivateKey
}

func CreateAuthRepo(db *ormKit.DB, accessTokenKeyPath, refreshTokenKeyPath string) (domain.AuthRepo, error) {
	accessTokenKey, err := parsePemKey(accessTokenKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, "parse pem key failed")
	}
	refreshTokenKey, err := parsePemKey(refreshTokenKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, "parse pem key failed")
	}

	return &authRepo{
		db:              db,
		accessTokenKey:  accessTokenKey,
		refreshTokenKey: refreshTokenKey,
	}, nil
}

func (a *authRepo) GenerateToken(sub string, iat, exp time.Time) (string, error) {
	accessToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": sub,
		"iat": iat.Unix(),
		"exp": exp.Unix(),
	})
	signedToken, err := accessToken.SignedString(a.accessTokenKey)
	if err != nil {
		return "", errors.Wrap(err, "signed access token failed")
	}
	return signedToken, nil
}

func (a *authRepo) CreateToken(userID int64, token string, expireAt time.Time, tokenType domain.AccountTokenEnum) (*domain.AccountToken, error) {
	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return nil, errors.Wrap(err, "generate unique id failed")
	}

	now := time.Now()

	tokenInstance := accountTokenEntity{
		AccountToken: &domain.AccountToken{
			ID:        uniqueIDGenerate.Generate().GetInt64(),
			Token:     token,
			Type:      domain.REFRESH_TOKEN,
			UserID:    userID,
			Status:    domain.ACTIVE,
			ExpireAt:  expireAt,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := a.db.Create(&tokenInstance).Error; err != nil {
		return nil, errors.Wrap(err, "save refresh token failed")
	}

	return tokenInstance.AccountToken, nil
}

func (a *authRepo) GetLastRefreshTokenByUserID(userID int64) (*domain.AccountToken, error) {
	var refreshToken accountTokenEntity
	if err := a.db.Last(&refreshToken, "user_id = ?", userID); err != nil { // TODO: check user_id name
		return nil, errors.Wrap(err, "get refresh token failed")
	}
	return refreshToken.AccountToken, nil
}

func (a *authRepo) UpdateStatusToken(token string, status domain.AccountTokenStatusEnum) error {
	if err := a.db.Model(&accountTokenEntity{}).Where("token = ?", token).Updates(accountTokenEntity{
		&domain.AccountToken{
			Status:    status,
			UpdatedAt: time.Now(),
		},
	}).Error; err != nil {
		return errors.Wrap(err, "update refresh token failed")
	}
	return nil
}

func (a *authRepo) VerifyToken(token string, accountTokenEnum domain.AccountTokenEnum) (int64, error) {
	key := new(ecdsa.PrivateKey)
	switch accountTokenEnum {
	case domain.ACCESS_TOKEN:
		key = a.accessTokenKey
	case domain.REFRESH_TOKEN:
		key = a.refreshTokenKey
	default:
		return 0, errors.New("unknown token enum")
	}

	jwtToken, err := a.parseAndValidToken(token, key)
	if err != nil {
		return 0, errors.Wrap(err, "parse and valid token failed")
	}
	ok, err := verifyToken(jwtToken)
	if err != nil {
		return 0, errors.Wrap(err, "verify token failed")
	}
	if !ok {
		return 0, errors.Wrap(domain.ErrExpired, "token expired")
	}

	mapClaims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.Wrap(err, "verify token failed")
	}
	userID, ok := mapClaims["sub"]
	if !ok {
		return 0, errors.Wrap(err, "verify token failed")
	}
	userIDString, ok := userID.(string)
	if !ok {
		return 0, errors.Wrap(err, "verify token failed")
	}
	userIDInt, err := strconv.ParseInt(userIDString, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "verify token failed")
	}

	return userIDInt, nil
}

func (a *authRepo) parseAndValidToken(tokenString string, key *ecdsa.PrivateKey) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, errors.New(fmt.Sprintf("unexpected signing %s", token.Header["alg"]))
		}
		return &key.PublicKey, nil
	})
	if errors.Is(err, jwt.ErrTokenSignatureInvalid) || errors.Is(err, jwt.ErrTokenMalformed) {
		return nil, errors.Wrap(domain.ErrInvalidData, fmt.Sprintf("%+v", err))
	} else if errors.Is(err, jwt.ErrTokenExpired) {
		return nil, errors.Wrap(domain.ErrExpired, fmt.Sprintf("%+v", err))
	} else if err != nil {
		return nil, errors.Wrap(err, "parse token get error")
	}
	return token, nil
}

func verifyToken(token *jwt.Token) (bool, error) {
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if expire, ok := claims["exp"]; ok {
			switch val := expire.(type) {
			case float64:
				if int64(val) < time.Now().Unix() {
					return false, nil
				}
				return true, nil
			default:
				return false, errors.New("get unexpected exp type")
			}
		} else {
			return false, errors.New("get exp field failed")
		}
	}
	return false, errors.New("get claims failed")
}

func parsePemKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read file failed")
	}
	blk, _ := pem.Decode(data) // TODO: check rest
	token, err := x509.ParseECPrivateKey(blk.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "parse private key failed")
	}

	return token, err
}
