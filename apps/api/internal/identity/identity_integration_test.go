package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIdentityRepositoryIntegrationAndLeastPrivilege(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	owner, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("owner connection: %v", err)
	}
	defer owner.Close()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse API database URL: %v", err)
	}
	config.ConnConfig.User = "real_estate_api"
	config.ConnConfig.Password = "real_estate_api"
	apiPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("API role connection: %v", err)
	}
	defer apiPool.Close()
	repository := NewRepository(apiPool)
	email := fmt.Sprintf("integration-%d@example.com", time.Now().UnixNano())

	session, err := repository.Register(
		ctx, email, "Integration User", "integration-password", "go-test")
	if err != nil || session.Token == "" || session.User.Email != email {
		t.Fatalf("Register()=%+v error=%v", session, err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(),
			`DELETE FROM app_users WHERE email=$1`, email)
	})
	user, err := repository.Authenticate(ctx, session.Token)
	if err != nil || user.ID != session.User.ID {
		t.Fatalf("Authenticate()=%+v error=%v", user, err)
	}
	preferences := json.RawMessage(`{"theme":"dark","map":{"zoom":8}}`)
	saved, err := repository.SavePreferences(ctx, user.ID, preferences)
	if err != nil || string(saved) == "{}" {
		t.Fatalf("SavePreferences()=%s error=%v", saved, err)
	}
	loaded, err := repository.Preferences(ctx, user.ID)
	var loadedObject, savedObject map[string]any
	_ = json.Unmarshal(loaded, &loadedObject)
	_ = json.Unmarshal(saved, &savedObject)
	if err != nil || !reflect.DeepEqual(loadedObject, savedObject) {
		t.Fatalf("Preferences()=%s error=%v", loaded, err)
	}
	login, err := repository.Login(
		ctx, email, "integration-password", "go-test")
	if err != nil || login.Token == "" {
		t.Fatalf("Login()=%+v error=%v", login, err)
	}
	if _, err := repository.Login(ctx, email, "wrong-password", "go-test"); err != ErrInvalidCredentials {
		t.Fatalf("invalid login error=%v", err)
	}
	if err := repository.Logout(ctx, session.Token); err != nil {
		t.Fatalf("Logout(): %v", err)
	}
	if _, err := repository.Authenticate(ctx, session.Token); err != ErrInvalidSession {
		t.Fatalf("logged-out authentication error=%v", err)
	}

	if _, err := apiPool.Exec(ctx,
		`UPDATE transactions SET notes=notes WHERE false`); err == nil {
		t.Fatal("API role unexpectedly has UPDATE permission on source transactions")
	}
	anomalyTransactionID := -time.Now().UnixNano()
	_, err = owner.Exec(ctx, `
		INSERT INTO analytics.anomaly_signals(
			transaction_kind,transaction_id,hard_rule_score,robust_cohort_score,
			duplicate_score,spatial_score,review_score,signal_version
		) VALUES ('sale',$1,0,0,0,0,0,'integration-test')`,
		anomalyTransactionID)
	if err != nil {
		t.Fatalf("insert anomaly permission fixture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `
			DELETE FROM analytics.anomaly_signals
			WHERE transaction_kind='sale' AND transaction_id=$1`,
			anomalyTransactionID)
	})
	tx, err := apiPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin review permission check: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `
		UPDATE analytics.anomaly_signals
		SET review_status='confirmed_valid'
		WHERE transaction_kind='sale' AND transaction_id=$1`,
		anomalyTransactionID)
	if err != nil || command.RowsAffected() != 1 {
		t.Fatalf("API role anomaly review permission rows=%d error=%v",
			command.RowsAffected(), err)
	}
}
