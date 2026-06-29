package auth

import (
	"testing"

	"echo_chat_server/internal/model"
)

func TestGenerateTokenPairAndParseByType(t *testing.T) {
	user := model.UserInfo{
		Id:        1,
		Uuid:      "U2026032600001",
		Telephone: "13800138000",
		IsAdmin:   1,
	}

	tokenPair, err := GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("GenerateTokenPair returned error: %v", err)
	}
	if tokenPair.SessionID == "" {
		t.Fatalf("expected session id to be generated")
	}

	accessClaims, err := ParseTokenAs(tokenPair.AccessToken, AccessTokenType)
	if err != nil {
		t.Fatalf("ParseTokenAs access token returned error: %v", err)
	}
	if accessClaims.SessionID != tokenPair.SessionID {
		t.Fatalf("expected access token session id %s, got %s", tokenPair.SessionID, accessClaims.SessionID)
	}
	if accessClaims.UserUUID != user.Uuid {
		t.Fatalf("expected access token user uuid %s, got %s", user.Uuid, accessClaims.UserUUID)
	}

	refreshClaims, err := ParseTokenAs(tokenPair.RefreshToken, RefreshTokenType)
	if err != nil {
		t.Fatalf("ParseTokenAs refresh token returned error: %v", err)
	}
	if refreshClaims.SessionID != tokenPair.SessionID {
		t.Fatalf("expected refresh token session id %s, got %s", tokenPair.SessionID, refreshClaims.SessionID)
	}

	if _, err := ParseTokenAs(tokenPair.RefreshToken, AccessTokenType); err == nil {
		t.Fatalf("expected refresh token to fail access token type validation")
	}
}
