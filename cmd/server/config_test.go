package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	t.Run("full config", func(t *testing.T) {
		data := `{
			"port": 9090,
			"roles": ["tow_driver", "tow_subscriber"],
			"channels": [
				{"name": "GPS_REALTIME", "roles": ["tow_driver", "tow_subscriber"]}
			]
		}`
		os.WriteFile(path, []byte(data), 0644)

		cfg, err := LoadConfig(path)
		require.NoError(t, err)
		require.Equal(t, 9090, cfg.Port)
		require.Equal(t, []string{"tow_driver", "tow_subscriber"}, cfg.Roles)
		require.Len(t, cfg.Channels, 1)
		require.Equal(t, "GPS_REALTIME", cfg.Channels[0].Name)
	})

	t.Run("default port when missing", func(t *testing.T) {
		data := `{"roles": [], "channels": []}`
		os.WriteFile(path, []byte(data), 0644)

		cfg, err := LoadConfig(path)
		require.NoError(t, err)
		require.Equal(t, 8080, cfg.Port)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/config.json")
		require.Error(t, err)
	})
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{bad"), 0644)

	_, err := LoadConfig(path)
	require.Error(t, err)
}
