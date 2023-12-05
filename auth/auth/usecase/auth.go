package usecase

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/auth/auth/repository"
	"github.com/superj80820/system-design/auth/domain"
	"github.com/superj80820/system-design/kit/code"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	mysqlKit "github.com/superj80820/system-design/kit/mysql"
	utilKit "github.com/superj80820/system-design/kit/util"
)

const (
	_ACCESS_TOKEN_KEY_PATH  = "./access-private-key.pem"
	_REFRESH_TOKEN_KEY_PATH = "./refresh-private-key.pem"
)

type AuthService struct {
	db     *mysqlKit.DB
	logger *loggerKit.Logger

	accessTokenKey  *ecdsa.PrivateKey
	refreshTokenKey *ecdsa.PrivateKey
}

func CreateAuthService(db *mysqlKit.DB, logger *loggerKit.Logger) (*AuthService, error) {
	if db == nil || logger == nil {
		return nil, errors.New("create service failed")
	}
	accessTokenKey, err := parsePemKey(_ACCESS_TOKEN_KEY_PATH)
	if err != nil {
		return nil, errors.Wrap(err, "parse pem key failed")
	}
	refreshTokenKey, err := parsePemKey(_REFRESH_TOKEN_KEY_PATH)
	if err != nil {
		return nil, errors.Wrap(err, "parse pem key failed")
	}

	return &AuthService{
		db:              db,
		logger:          logger,
		accessTokenKey:  accessTokenKey,
		refreshTokenKey: refreshTokenKey,
	}, nil
}

func (a *AuthService) Login(email, password string) (*domain.Account, error) {
	var account repository.AccountEntity
	err := a.db.First(&account, "email = ?", email)
	if errors.Is(err, mysqlKit.ErrRecordNotFound) {
		return nil, code.CreateErrorCode(http.StatusUnauthorized)
	} else if err != nil {
		return nil, errors.Wrap(err, "get db user failed")
	}

	if account.Password != utilKit.GetSHA256(password) {
		return nil, code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.PasswordInvalid)
	}

	now := time.Now()

	refreshTokenExpireAt := now.Add(time.Hour * 12)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{ // TODO: need ecdsa, or hmac?
		"sub": account.ID,
		"iat": now.Unix(),
		"exp": refreshTokenExpireAt.Unix(),
	})
	signedRefreshToken, err := refreshToken.SignedString(a.refreshTokenKey)
	if err != nil {
		return nil, errors.Wrap(err, "signed refresh token failed")
	}

	accessTokenExpireAt := now.Add(time.Hour * 1) // TODO: test 失效
	accessToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": account.ID,
		"iat": now.Unix(),
		"exp": accessTokenExpireAt.Unix(),
	})
	signedAccessToken, err := accessToken.SignedString(a.accessTokenKey)
	if err != nil {
		return nil, errors.Wrap(err, "signed access token failed")
	}

	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return nil, errors.Wrap(err, "generate unique id failed")
	}

	if err := a.db.Create(&repository.AccountTokenEntity{
		ID:        uniqueIDGenerate.Generate().GetInt64(),
		Token:     signedRefreshToken,
		Type:      domain.REFRESH_TOKEN,
		UserID:    account.ID,
		Status:    domain.ACTIVE,
		ExpireAt:  refreshTokenExpireAt,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, errors.Wrap(err, "save refresh token failed")
	}

	return &domain.Account{
		Email:        email,
		Password:     password,
		AccessToken:  signedAccessToken,
		RefreshToken: signedRefreshToken,
	}, nil
}

func (a *AuthService) Logout(accessToken string) error {
	token, err := a.parseAndValidToken(accessToken, a.accessTokenKey)
	if err != nil {
		return err
	}
	ok, err := verifyToken(token)
	if err != nil {
		return errors.Wrap(err, "verify token failed")
	}
	if !ok {
		return code.CreateErrorCode(http.StatusUnauthorized)
	}

	userID, err := getUserIDToken(token)
	if err != nil {
		return errors.Wrap(err, "get user id from token failed")
	}

	var refreshToken repository.AccountTokenEntity
	if err := a.db.Last(&refreshToken, "user_id = ?", userID); err != nil { // TODO: check user_id name
		return errors.Wrap(err, "get refresh token failed")
	}

	refreshToken.Status = domain.REVOKE
	refreshToken.UpdatedAt = time.Now()
	if err := a.db.Save(&refreshToken); err != nil {
		return errors.Wrap(err, "update refresh token failed")
	}

	return nil
}

func (a *AuthService) RefreshAccessToken(refreshTokenString string) (string, error) {
	token, err := a.parseAndValidToken(refreshTokenString, a.refreshTokenKey)
	if err != nil {
		return "", err
	}
	ok, err := verifyToken(token)
	if err != nil {
		return "", errors.Wrap(err, "verify token failed")
	}
	if !ok {
		return "", code.CreateErrorCode(http.StatusUnauthorized)
	}

	userID, err := getUserIDToken(token)
	if err != nil {
		return "", errors.Wrap(err, "get user id from token failed")
	}

	var refreshToken repository.AccountTokenEntity
	if err := a.db.Last(&refreshToken, "user_id = ?", userID); err != nil { // TODO: check user_id name
		return "", errors.Wrap(err, "get refresh token failed")
	}

	if time.Now().After(refreshToken.ExpireAt) {
		return "", code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Expired)
	}

	if refreshToken.Status == domain.REVOKE {
		return "", code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Revoke)
	}

	now := time.Now()

	accessTokenExpireAt := now.Add(time.Hour * 1)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": userID,
		"iat": strconv.FormatInt(now.Unix(), 10),
		"exp": strconv.FormatInt(accessTokenExpireAt.Unix(), 10), // TODO: test len
	})
	signedAccessToken, err := accessToken.SignedString(a.accessTokenKey)
	if err != nil {
		return "", errors.Wrap(err, "signed access token failed")
	}

	return signedAccessToken, nil
}

func (a *AuthService) Verify(accessToken string) error {
	token, err := a.parseAndValidToken(accessToken, a.accessTokenKey)
	if err != nil {
		return err
	}
	ok, err := verifyToken(token)
	if err != nil {
		return errors.Wrap(err, "verify token failed")
	}
	if !ok {
		return code.CreateErrorCode(http.StatusUnauthorized)
	}
	return nil
}

func (a *AuthService) parseAndValidToken(tokenString string, key *ecdsa.PrivateKey) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, errors.New(fmt.Sprintf("unexpected signing %s", token.Header["alg"]))
		}
		return &key.PublicKey, nil
	})
	if errors.Is(err, jwt.ErrTokenSignatureInvalid) || errors.Is(err, jwt.ErrTokenMalformed) {
		return nil, code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.PasswordInvalid).AddErrorMetaData(err)
	} else if errors.Is(err, jwt.ErrTokenExpired) {
		return nil, code.CreateErrorCode(http.StatusUnauthorized).AddCode(code.Expired).AddErrorMetaData(err)
	} else if err != nil {
		return nil, errors.Wrap(err, "parse token get error")
	}
	return token, nil
}

// TODO: use generic
func getUserIDToken(token *jwt.Token) (int64, error) {
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if userID, ok := claims["sub"]; ok {
			switch val := userID.(type) {
			case float64:
				return int64(val), nil // TODO: check int64 type correct
			default:
				return 0, errors.New("get unexpected sub type")
			}
		} else {
			return 0, errors.New("get sub field failed")
		}
	}
	return 0, errors.New("not found user id")
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
