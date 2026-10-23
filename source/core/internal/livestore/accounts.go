package livestore

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"docssearch/core/internal/texts"
)

func (store *LiveStore) Login_User(ctx context.Context, loginName, password, token string) (LoginResult, error) {
	var result LoginResult
	var passwordHash string
	err := store.pool.QueryRow(ctx, `
		SELECT user_id, display_name, access_level, password_hash
		FROM users WHERE login = $1`, loginName).Scan(&result.UserID, &result.DisplayName, &result.AccessLevel, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil) {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO auth_tokens (token, user_id) VALUES ($1, $2)`, token, result.UserID); err != nil {
		return LoginResult{}, err
	}
	result.Token = token
	return result, nil
}

func (store *LiveStore) Delete_AuthToken(ctx context.Context, token string) error {
	tag, err := store.pool.Exec(ctx, `DELETE FROM auth_tokens WHERE token = $1`, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUnauthorized
	}
	return nil
}

func (store *LiveStore) Find_AccountByToken(ctx context.Context, token string) (Account, error) {
	var account Account
	err := store.pool.QueryRow(ctx, `
		SELECT u.user_id, u.login, u.display_name, u.access_level
		FROM auth_tokens t
		JOIN users u ON u.user_id = t.user_id
		WHERE t.token = $1`, token).Scan(&account.UserID, &account.Login, &account.DisplayName, &account.AccessLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrUnauthorized
	}
	return account, err
}

func (store *LiveStore) List_Accounts(ctx context.Context, token string) ([]Account, error) {
	if _, err := store.require_Admin(ctx, token); err != nil {
		return nil, err
	}
	rows, err := store.pool.Query(ctx, `
		SELECT user_id, login, display_name, access_level
		FROM users
		ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []Account{}
	for rows.Next() {
		var account Account
		if err := rows.Scan(&account.UserID, &account.Login, &account.DisplayName, &account.AccessLevel); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (store *LiveStore) Create_Account(ctx context.Context, token, loginName, password, displayName string, accessLevel int) (Account, error) {
	if _, err := store.require_Admin(ctx, token); err != nil {
		return Account{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, err
	}
	number, err := store.next_ID(ctx, counterUser)
	if err != nil {
		return Account{}, err
	}
	userID := "user-" + strconv.Itoa(number)
	_, err = store.pool.Exec(ctx, `
		INSERT INTO users (user_id, login, password_hash, display_name, access_level)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, loginName, string(hash), displayName, accessLevel)
	if is_UniqueViolation(err) {
		return Account{}, ErrLoginTaken
	}
	if err != nil {
		return Account{}, err
	}
	return Account{UserID: userID, Login: loginName, DisplayName: displayName, AccessLevel: accessLevel}, nil
}

func (store *LiveStore) Patch_Account(ctx context.Context, token, userID string, displayName *string, accessLevel *int) (Account, error) {
	if _, err := store.require_Admin(ctx, token); err != nil {
		return Account{}, err
	}
	if displayName == nil && accessLevel == nil {
		return Account{}, ErrNotFound
	}
	account, err := store.find_AccountByID(ctx, userID)
	if err != nil {
		return Account{}, err
	}
	if displayName != nil {
		account.DisplayName = strings.TrimSpace(*displayName)
	}
	if accessLevel != nil {
		account.AccessLevel = *accessLevel
	}
	_, err = store.pool.Exec(ctx, `
		UPDATE users SET display_name = $2, access_level = $3 WHERE user_id = $1`,
		account.UserID, account.DisplayName, account.AccessLevel)
	return account, err
}

func (store *LiveStore) require_Admin(ctx context.Context, token string) (Account, error) {
	account, err := store.Find_AccountByToken(ctx, token)
	if err != nil {
		return Account{}, err
	}
	if account.AccessLevel < texts.SeedAdminAccessLevel {
		return Account{}, ErrAccessDenied
	}
	return account, nil
}

func (store *LiveStore) find_AccountByID(ctx context.Context, userID string) (Account, error) {
	var account Account
	err := store.pool.QueryRow(ctx, `
		SELECT user_id, login, display_name, access_level FROM users WHERE user_id = $1`, userID).
		Scan(&account.UserID, &account.Login, &account.DisplayName, &account.AccessLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return account, err
}

func is_UniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}
