package couchbasecapella

import (
	"context"
	"flag"
	"testing"
	"time"

	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
	"github.com/labstack/gommon/random"
)

var (
	apiUrl             string
	orgId              string
	projectId          string
	clusterId          string
	adminUserAccessKey string
	adminUserSecretKey string
	initReq            dbplugin.InitializeRequest
)

func init() {
	flag.StringVar(&apiUrl, "apiUrl", "https://cloudapi.dev.nonprod-project-avengers.com/v4", "The base URL for the Couchbase Capella Cloud API")
	flag.StringVar(&orgId, "orgId", "", "The organization ID for the Couchbase Capella Cloud API")
	flag.StringVar(&projectId, "projectId", "", "The project ID for the Couchbase Capella Cloud API")
	flag.StringVar(&clusterId, "clusterId", "", "The cluster ID for the Couchbase Capella Cloud API")
	flag.StringVar(&adminUserAccessKey, "adminUserAccessKey", "", "The admin user apiKey for the Couchbase Capella Cloud API")
	flag.StringVar(&adminUserSecretKey, "adminUserSecretKey", "", "The admin user secretKey for the Couchbase Capella Cloud API")
}

var connectionDetails = map[string]interface{}{
	"plugin_name":     "couchbasecapella-database-plugin",
	"password_policy": "couchbasecapella",
	"allowed_roles":   "*",
}

var testCouchbaseCapellaRole = `{"access": [
		{
		  "privileges": [
			"data_reader"
		  ],
		  "resources": {
			"buckets": [
			  {
				"name": "*"
				}
			]
		}
		}]}`

func setupCouchbaseCapellaDBInitialize(t *testing.T) (*CouchbaseCapellaDB, error) {
	initReq = dbplugin.InitializeRequest{
		Config:           connectionDetails,
		VerifyConnection: false,
	}

	db := new()

	_, err := db.Initialize(context.Background(), initReq)
	if err != nil {
		return nil, err
	}

	if !db.Initialized {
		t.Fatal("Database should be initialized")
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	return db, nil
}

func doCouchbaseCapellaDBSetCredentials(t *testing.T, username, password string) {
	t.Logf("Testing SetCredentials(%s)", username)

	db, err := setupCouchbaseCapellaDBInitialize(t)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	updateReq := dbplugin.UpdateUserRequest{
		Username: username,
		Password: &dbplugin.ChangePassword{
			NewPassword: password,
		},
	}

	_, err = db.UpdateUser(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	db.Close()

}

func doCouchbaseCapellaDBNewCredentials(t *testing.T, username, password, rolename string) dbplugin.NewUserResponse {
	t.Logf("Testing NewCredentials(%s,%s,%s)", username, rolename, password)

	db, err := setupCouchbaseCapellaDBInitialize(t)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: username,
			RoleName:    rolename,
		},
		Statements: dbplugin.Statements{
			Commands: []string{testCouchbaseCapellaRole},
		},
		Password:   password,
		Expiration: time.Now().Add(time.Minute),
	}

	userResp, err := db.NewUser(context.Background(), createReq)
	if err != nil {
		t.Fatalf("err: %s", err)
	} else {
		t.Logf("Username: %s", userResp.Username)
	}

	db.Close()

	return userResp

}

func revokeUser(t *testing.T, username string) error {
	t.Log("Testing RevokeUser()")

	db, err := setupCouchbaseCapellaDBInitialize(t)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	delUserReq := dbplugin.DeleteUserRequest{Username: username}

	_, err = db.DeleteUser(context.Background(), delUserReq)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	return nil
}

func TestDriver(t *testing.T) {

	// Check if the required flags are set
	if len(apiUrl) == 0 {
		t.Fatal("apiUrl cannot be empty")
	}
	if len(orgId) == 0 {
		t.Fatal("orgId cannot be empty. Set it through the env variable ORG_ID or -orgId flag")
	}
	if len(projectId) == 0 {
		t.Fatal("projectId cannot be empty. Set it through the env variable PROJECT_ID or -projectId flag")
	}
	if len(clusterId) == 0 {
		t.Fatal("clusterId cannot be empty. Set it through the env variable CLUSTER_ID or -clusterId flag")
	}
	if len(adminUserAccessKey) == 0 {
		t.Fatal("adminUserAccessKey cannot be empty. Set it through the env variable ADMIN_USER_ACCESS_KEY or -adminUserAccessKey flag")
	}
	if len(adminUserSecretKey) == 0 {
		t.Fatal("adminUserSecretKey cannot be empty. Set it through the env variable ADMIN_USER_SECRET_KEY or -adminUserSecretKey flag")
	}

	// Set up the connection details
	connectionDetails["cloud_api_base_url"] = apiUrl
	connectionDetails["organization_id"] = orgId
	connectionDetails["project_id"] = projectId
	connectionDetails["cluster_id"] = clusterId
	connectionDetails["username"] = adminUserAccessKey
	connectionDetails["password"] = adminUserSecretKey

	t.Run("Creds", func(t *testing.T) { testCouchbaseCapellaDBSetCredentials(t) })
	t.Run("Secret", func(t *testing.T) { testConnectionProducerSecretValues(t) })
	t.Run("Create/long username", func(t *testing.T) { testCreateuser_UsernameTemplate_LongUsername(t) })
	t.Run("Create/custom username template", func(t *testing.T) { testCreateUser_UsernameTemplate_CustomTemplate(t) })
	t.Run("Rotate", func(t *testing.T) { testCouchbaseCapellaDBRotateRootCredentials(t) })

}

func testCouchbaseCapellaDBInitialize(t *testing.T) {
	t.Log("Testing DB Init()")

	setupCouchbaseCapellaDBInitialize(t)
}

func testCouchbaseCapellaDBCreateUser(t *testing.T) {
	t.Log("Testing CreateUser()")

	username := "test"
	password := "MQlbO5zbTX1gmn!%rbMfGhJrWqI6Vi8irMGX5lW!hZyF0vBj@lNILU!Y#vnVnaDn"
	doCouchbaseCapellaDBNewCredentials(t, username, password, username)

}

func testCouchbaseCapellaDBCreateAndRevokeUser(t *testing.T) {
	t.Log("Testing Create and RevokeUser()")
	username := "test"
	password := "MQlbO5zbTX1gmn!%rbMfGhJrWqI6Vi8irMGX5lW!hZyF0vBj@lNILU!Y#vnVnaDn"
	userResp := doCouchbaseCapellaDBNewCredentials(t, username, password, username)

	err := revokeUser(t, userResp.Username)
	if err != nil {
		t.Fatalf("Could not revoke user: %s", userResp.Username)
	}
}

func testCouchbaseCapellaDBCreateUser_DefaultRole(t *testing.T, address string, port int) {
	t.Log("Testing CreateUser_DefaultRole()")

	db, err := setupCouchbaseCapellaDBInitialize(t)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	username := "test3"
	password := "lmT9a61EmAD@uaWDYSXFtzJ7DLnaexljL9zqQL6vUiVrGVQOufMzRPG1D6OmJT3W"

	userResp := doCouchbaseCapellaDBNewCredentials(t, username, password, username)

	err = revokeUser(t, userResp.Username)
	if err != nil {
		t.Fatalf("Could not revoke user: %s", username)
	}

	db.Close()
}

func testCouchbaseCapellaDBCreateUser_plusRole(t *testing.T) {
	t.Log("Testing CreateUser_plusRole()")

	password := "Cac%S97Enm34hcELxT5wjBgYkCSpEpSp%KeTgnrLX!wbo@ugROzgswxP2a1zOGz@"

	username := "test2"
	userResp := doCouchbaseCapellaDBNewCredentials(t, username, password, username)

	err := revokeUser(t, userResp.Username)
	if err != nil {
		t.Fatalf("Could not revoke user: %s", userResp.Username)
	}
}

func testCouchbaseCapellaDBRotateRootCredentials(t *testing.T) {
	t.Log("Testing RotateRootCredentials()")

	db, err := setupCouchbaseCapellaDBInitialize(t)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	rotatePassword := "eNJMuUwHP95vxSftiBsjO2WPS1znWcIQlng64PKIkUjCf5#yBVWDS8tFtFnGt7es"
	updateReq := dbplugin.UpdateUserRequest{
		Username: adminUserAccessKey,
		Password: &dbplugin.ChangePassword{
			NewPassword: rotatePassword,
		},
	}

	_, err = db.UpdateUser(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	t.Logf("New adminUserAccessKey: %s", rotatePassword)

	// rotate back
	t.Logf("Rotating back to original adminUserAccessKey: %s", adminUserSecretKey)
	rotatePasswordBack := adminUserSecretKey
	connectionDetails["password"] = rotatePassword
	adminUserSecretKey = rotatePassword
	db, err = setupCouchbaseCapellaDBInitialize(t)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	updateReq = dbplugin.UpdateUserRequest{
		Username: adminUserAccessKey,
		Password: &dbplugin.ChangePassword{
			NewPassword: rotatePasswordBack,
		},
	}

	_, err = db.UpdateUser(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	adminUserSecretKey = rotatePasswordBack
	connectionDetails["password"] = rotatePasswordBack

	db.Close()

}

func testCouchbaseCapellaDBSetCredentials(t *testing.T) {
	newUserResp := doCouchbaseCapellaDBNewCredentials(t, "vault-edu", "sKhIJRBa0q1GMk9#4Hh7UyJ0ZO88q7k4s1RhFKj3Rh0tKNmIYLYaRRWX3dHbTelW", "vault-edu")
	doCouchbaseCapellaDBSetCredentials(t, newUserResp.Username, "isxsV0WS%ZQ@!smVrBum46W2B02uAJy#bFiuRNOLtL21t8%KYTIjvz6vZl9A0Ao8")
}

func testConnectionProducerSecretValues(t *testing.T) {
	t.Log("Testing CouchbaseCapellaDBConnectionProducer.secretValues()")

	cp := &couchbaseCapellaDBConnectionProducer{
		Username: "USR",
		Password: "PWD",
	}

	if cp.secretValues()["USR"] != "[username]" &&
		cp.secretValues()["PWD"] != "[password]" {
		t.Fatal("CouchbaseCapellaDBConnectionProducer.secretValues() test failed.")
	}
}

func testCreateUser_UsernameTemplate_CustomTemplate(t *testing.T) {

	password := "wG02OMyi3216mdg6Jbxi8Av5w2qK6@yakvzlIZARfyn0Dy048@zsRLnDoONHAhwZ"
	username := "token"
	doCouchbaseCapellaDBNewCredentials(t, username, password, "thisrolename")

}

func testCreateuser_UsernameTemplate_LongUsername(t *testing.T) {

	db, err := setupCouchbaseCapellaDBInitialize(t)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer dbtesting.AssertClose(t, db)

	dbtesting.AssertInitialize(t, db, initReq)

	username := "thisissomereallylongdisplaynameforthetemplate" + random.String(4, random.Alphanumeric)
	rolename := "thisissomereallylongrolenameforthetemplate" + random.String(4, random.Alphanumeric)

	password := "MFNdINEHyXo4cJFcjxQ!bHq%@Gnoi2hyOT5sOXvxjUnWfqrnMeW98H6Uu%MiBn0V"
	doCouchbaseCapellaDBNewCredentials(t, username, password, rolename)
}
