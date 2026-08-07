package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/auth"
)

func TestUserCLICommands(t *testing.T) {
	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.yaml")

	t.Run("User Add Command", func(t *testing.T) {
		cmd := userAddCmd
		err := cmd.Flags().Set("username", "alice")
		require.NoError(t, err)
		err = cmd.Flags().Set("email", "alice@company.com")
		require.NoError(t, err)
		err = cmd.Flags().Set("groups", "DevOps,SecOps")
		require.NoError(t, err)
		err = cmd.Flags().Set("users-file", usersFile)
		require.NoError(t, err)

		err = runUserAdd(cmd, nil)
		require.NoError(t, err)

		la, err := auth.NewLocalAuthenticator(usersFile)
		require.NoError(t, err)
		users := la.ListUsers()
		require.Len(t, users, 1)
		assert.Equal(t, "alice", users[0].Username)
		assert.Equal(t, "local", users[0].Provider)
		assert.Equal(t, []string{"DevOps", "SecOps"}, users[0].Groups)
		assert.NotEmpty(t, users[0].Hash)
	})

	t.Run("User List Command", func(t *testing.T) {
		cmd := userListCmd
		err := cmd.Flags().Set("users-file", usersFile)
		require.NoError(t, err)

		err = runUserList(cmd, nil)
		require.NoError(t, err)
	})

	t.Run("User Disable Command", func(t *testing.T) {
		cmd := userDisableCmd
		err := cmd.Flags().Set("username", "alice")
		require.NoError(t, err)
		err = cmd.Flags().Set("users-file", usersFile)
		require.NoError(t, err)

		err = runUserDisable(cmd, nil)
		require.NoError(t, err)

		la, err := auth.NewLocalAuthenticator(usersFile)
		require.NoError(t, err)
		users := la.ListUsers()
		require.Len(t, users, 1)
		assert.True(t, users[0].Disabled)
	})

	t.Run("User Delete Command", func(t *testing.T) {
		cmd := userDeleteCmd
		err := cmd.Flags().Set("username", "alice")
		require.NoError(t, err)
		err = cmd.Flags().Set("users-file", usersFile)
		require.NoError(t, err)

		err = runUserDelete(cmd, nil)
		require.NoError(t, err)

		la, err := auth.NewLocalAuthenticator(usersFile)
		require.NoError(t, err)
		users := la.ListUsers()
		assert.Len(t, users, 0)
	})
}
