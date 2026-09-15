package services

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/apperrors"
	"github.com/dronm/meatshop/internal/models"

	"github.com/dronm/session"
	"github.com/dronm/webapp"
)

const (
	userLoginPubKeyQuery = "USER_LOGIN_PUB_KEY_QUERY"
	userLoginInsertQuery = "USER_LOGIN_INSERT_QUERY"
	userLoginUpdateQuery = "USER_LOGIN_UPDATE_QUERY"

	pubKeyLen = 15
)

func loginUser(
	ctx context.Context,
	sess session.Session,
	userInf models.UserLoginInf,
	conn ds.Conn,
	userRow *models.UserLogin,
) error {
	if sess == nil {
		return webapp.Unauthorized("session is required", nil)
	}

	if err := prepareUserLoginSQL(ctx, conn); err != nil {
		return err
	}

	if userRow.Banned {
		return apperrors.UserBanned()
	}

	sessID := sess.SessionID()

	// user agent
	var err error
	var userHeaders []byte
	userAgentField := []byte("{}")

	userIP := userInf.RemoteAddr
	userIPPortPos := strings.Index(userIP, ":")
	if userIPPortPos >= 0 {
		userIP = userIP[:userIPPortPos]
	}
	userHeaders, err = json.Marshal(userInf.Headers)
	if err != nil {
		slog.Error("json.Marshal()", "headers", userInf.Headers, "err", err)
	}

	userAgent := userInf.Headers.Get("User-Agent")
	if userAgent != "" {
		userAgentField, err = userAgentFieldValue(userAgent)
		if err != nil {
			slog.Error("userAgentFieldValue()", "userAgent", userAgent, "err", err)
		}
	}

	if userRow.RoleID != models.RoleIDAdmin {
		deviceHashBytes := md5.Sum(userAgentField)
		deviceHash := hex.EncodeToString(deviceHashBytes[:])
		if userRow.BanHash != nil && strings.Contains(*userRow.BanHash, deviceHash) {
			return apperrors.UserDeviceBanned()
		}
	}

	err = conn.QueryRow(ctx, userLoginPubKeyQuery, sessID, userRow.ID).Scan(
		&userRow.PubKey,
		&userRow.LoginID,
	)

	if err == ds.ErrNoRows {
		// no user login
		userRow.PubKey, err = genPublicKey()
		if err != nil {
			return fmt.Errorf("genPublicKey() failed: %v", err)
		}

		err := conn.QueryRow(ctx, userLoginUpdateQuery,
			userRow.ID,
			userRow.PubKey,
			string(userAgentField), userIP, string(userHeaders),
			sessID,
		).Scan(&userRow.LoginID)

		// fmt.Println("userIP:", userIP)
		// fmt.Println("userPubKey:", userRow.PubKey)
		if err == ds.ErrNoRows {
			// no user
			if err = conn.QueryRow(ctx,
				userLoginInsertQuery,
				userIP,
				sessID,
				userRow.PubKey,
				userRow.ID,
				string(userAgentField),
				string(userHeaders),
			).Scan(&userRow.LoginID); err != nil {
				return fmt.Errorf("users_login_insert pgx.Rows.Scan(): %v", err)
			}
		} else if err != nil {
			return fmt.Errorf("users_login_update pgx.Rows.Scan(): %v", err)
		}

	} else if err != nil {
		return fmt.Errorf("USER_LOGIN_PUB_KEY_Q pgx.Rows.Scan(): %v", err)
	}

	// Session data
	gob.Register(models.UserLogin{})

	if err := sess.Set("user", *userRow); err != nil {
		return fmt.Errorf("sess.Set('user'): %v", err)
	}

	if err := sess.Flush(); err != nil {
		return fmt.Errorf("sess.Flush(): %v", err)
	}
	slog.Debug("Flushed a session for a logged user", "sess", sess, "userData", *userRow)
	//******* Preset filters for all users*******************

	return nil
}

func prepareUserLoginSQL(ctx context.Context, conn ds.Conn) error {
	if _, err := conn.Prepare(ctx, userLoginPubKeyQuery,
		`SELECT pub_key, id
		FROM logins
		WHERE session_id=$1 AND user_id=$2 AND date_time_out IS NULL`); err != nil {
		return fmt.Errorf("USER_LOGIN_PUB_KEY_Q pgx.Conn.Prepare(): %v", err)
	}

	if _, err := conn.Prepare(context.Background(),
		userLoginUpdateQuery,
		`UPDATE logins
		SET
			user_id = $1,
			pub_key = $2,
			date_time_in = now(),
			set_date_time = now(),
			user_agent = $3::jsonb,
			ip = $4,
			headers = $5::json
			FROM (
				SELECT
					l.id AS id
				FROM logins l
				WHERE l.session_id=$6 AND l.user_id IS NULL
				ORDER BY l.date_time_in DESC
				LIMIT 1										
			) AS s
			WHERE s.id = logins.id
		RETURNING logins.id`); err != nil {
		return fmt.Errorf("userLoginInsertQuery pgx.Conn.Prepare(): %w", err)
	}

	if _, err := conn.Prepare(context.Background(),
		userLoginInsertQuery,
		`INSERT INTO logins
		(date_time_in, ip, session_id, pub_key, user_id, user_agent, headers)
		VALUES (now(), $1, $2, $3, $4, $5, $6)				
		RETURNING id`); err != nil {
		return fmt.Errorf("userLoginInsertQuery pgx.Conn.Prepare(): %w", err)
	}

	return nil
}

func genPublicKey() (string, error) {
	sessionID := make([]byte, pubKeyLen/2)
	_, err := rand.Read(sessionID)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sessionID), nil
}
