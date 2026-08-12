package versioning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreeze(t *testing.T) {
	InvalidateLoaderCache()
	root := t.TempDir()
	api := filepath.Join(root, "api")
	hubUser := filepath.Join(api, "hub", "user")
	err := os.MkdirAll(hubUser, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	proto := `syntax = "proto3";
package user.hub;
option go_package = "example/gen/api/hub/user;userhub";
service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse) {
    option (google.api.http) = {get: "/users/{id}"};
  }
}
message GetUserRequest { string id = 1; }
message GetUserResponse { string id = 1; }
`
	err = os.WriteFile(filepath.Join(hubUser, "service.proto"), []byte(proto), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// DiscoverHubMeta reads generated Go; provide a minimal hub package for freeze metadata.
	genHub := filepath.Join(root, "gen", "api", "hub", "user")
	err = os.MkdirAll(genHub, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	stub := `package userhub

import "context"

type UserServiceServer interface {
	GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error)
	mustEmbedUnimplementedUserServiceServer()
}
type GetUserRequest struct{}
type GetUserResponse struct{}
type UnimplementedUserServiceServer struct{}
func (UnimplementedUserServiceServer) GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error) {
	return nil, nil
}
func (UnimplementedUserServiceServer) mustEmbedUnimplementedUserServiceServer() {}
`
	err = os.WriteFile(filepath.Join(genHub, "stub.go"), []byte(stub), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	err = Freeze("api", "v2", "example", "gen")
	if err != nil {
		t.Fatal(err)
	}
	var out []byte

	out, err = os.ReadFile(filepath.Join(api, "v2", "user", "service.proto"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(out), "package user.v2;") {
		t.Fatalf("expected frozen package user.v2, got:\n%s", out)
	}
	if !contains(string(out), `get: "/v2/users/{id}"`) {
		t.Fatalf("expected versioned http path, got:\n%s", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
