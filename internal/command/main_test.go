package command

import (
	"os"
	"testing"

	"memkv/internal/logger"
)

func TestMain(m *testing.M) {
	logger.SetDefault(logger.Discard())
	os.Exit(m.Run())
}
