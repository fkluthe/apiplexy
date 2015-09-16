package sql

import (
	"database/sql"
	"fmt"
	"github.com/12foo/apiplexy"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"strings"
	"testing"
)

var plugin apiplexy.ManagementBackendPlugin

func TestMain(m *testing.M) {
	plugin = apiplexy.ManagementBackendPlugin(&SQLDBBackend{})
	err := plugin.Configure(map[string]interface{}{
		"driver":            "sqlite3",
		"connection_string": ":memory:",
		"create_tables":     true,
	})
	if err != nil {
		fmt.Printf("Couldn't initialize in-memory sqlite DB for testing. %s\n", err.Error())
		fmt.Printf("Available drivers: %s\n", strings.Join(sql.Drivers(), ", "))
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestConfigure(t *testing.T) {
	Convey("Plugin should not panic when configuring with default configuration", t, func() {
		So(func() {
			tplugin := apiplexy.ManagementBackendPlugin(&SQLDBBackend{})
			_ = tplugin.Configure(tplugin.DefaultConfig())
		}, ShouldNotPanic)
	})
}

func TestAddUser(t *testing.T) {
	testuser := apiplexy.User{
		Email: "test@user.com",
		Name:  "Test User",
	}
	initialPassword := "my-initial-password"

	err := plugin.AddUser(testuser.Email, initialPassword, &testuser)

	Convey("Adding a new user should not give an error", t, func() {
		So(err, ShouldBeNil)
	})

	Convey("The new user should be present and correctly filled out", t, func() {
		u := plugin.GetUser("test@user.com")
		So(u, ShouldNotBeNil)
		So(u.Name, ShouldEqual, "Test User")
	})

	Convey("Trying to add the same user twice should result in an error", t, func() {
		err = plugin.AddUser(testuser.Email, initialPassword, &testuser)
		So(err, ShouldNotBeNil)
	})
}

func TestActivateUser(t *testing.T) {
	Convey("An un-activated user should not be able to log in", t, func() {
		u := plugin.Authenticate("test@user.com", "my-initial-password")
		So(u, ShouldBeNil)
	})

	Convey("Activation should perform successfully", t, func() {
		u := plugin.GetUser("test@user.com")
		So(u, ShouldNotBeNil)
		So(u.Active, ShouldBeFalse)
		err := plugin.ActivateUser("test@user.com")
		u = plugin.GetUser("test@user.com")
		So(err, ShouldBeNil)
		So(u.Active, ShouldBeTrue)
	})

	Convey("An activated user should be able to log in", t, func() {
		u := plugin.Authenticate("test@user.com", "my-initial-password")
		So(u, ShouldNotBeNil)
	})
}

func TestResetPassword(t *testing.T) {
	Convey("Logging in with new password should fail before reset", t, func() {
		So(plugin.Authenticate("test@user.com", "new-password"), ShouldBeNil)
	})

	Convey("Resetting password should work", t, func() {
		u := plugin.GetUser("test@user.com")
		So(u, ShouldNotBeNil)
		So(plugin.ResetPassword("test@user.com", "new-password"), ShouldBeNil)
	})

	Convey("After reset, login with new password should work", t, func() {
		So(plugin.Authenticate("test@user.com", "new-password"), ShouldNotBeNil)
	})
}

func TestUpdateUser(t *testing.T) {
	Convey("Adding a field to user profile should work", t, func() {
		u := plugin.GetUser("test@user.com")
		u.Profile["pet"] = "dog"
		err := plugin.UpdateUser("test@user.com", u)
		So(err, ShouldBeNil)
	})

	Convey("The field should be present on subsequent requests", t, func() {
		u := plugin.GetUser("test@user.com")
		So(u.Profile["pet"], ShouldEqual, "dog")
	})
}

func TestKeyManagement(t *testing.T) {
	key := apiplexy.Key{
		ID:   "mykeyid",
		Type: "TestKey",
	}

	Convey("Adding a key to a nonexistant user should not work", t, func() {
		err := plugin.AddKey("not-there@user.com", &key)
		So(err, ShouldNotBeNil)
	})

	Convey("A new user should have 0 keys", t, func() {
		keys, err := plugin.GetAllKeys("test@user.com")
		So(err, ShouldBeNil)
		So(len(keys), ShouldEqual, 0)
	})

	Convey("Adding a key to an existing user should work", t, func() {
		err := plugin.AddKey("test@user.com", &key)
		So(err, ShouldBeNil)
	})

	Convey("The user should then have 1 key, matching the one just added", t, func() {
		keys, err := plugin.GetAllKeys("test@user.com")
		So(err, ShouldBeNil)
		So(len(keys), ShouldEqual, 1)
		So(keys[0].ID, ShouldEqual, key.ID)
	})

	Convey("The added key should be available for quota calculations", t, func() {
		k, err := plugin.GetKey("mykeyid", "TestKey")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		So(k.ID, ShouldEqual, key.ID)
	})

	Convey("Deleting a key the user does not own should not work", t, func() {
		So(plugin.DeleteKey("not-owner@user.com", "mykeyid"), ShouldNotBeNil)
	})

	Convey("Deleting a key the user owns should work", t, func() {
		So(plugin.DeleteKey("test@user.com", "mykeyid"), ShouldBeNil)
	})

	Convey("The user should then have 0 keys", t, func() {
		keys, err := plugin.GetAllKeys("test@user.com")
		So(err, ShouldBeNil)
		So(len(keys), ShouldEqual, 0)
	})

	Convey("The added key should not be available for anymore", t, func() {
		k, err := plugin.GetKey("mykeyid", "TestKey")
		So(err, ShouldBeNil)
		So(k, ShouldBeNil)
	})

}
